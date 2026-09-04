// Package telegram — источник Telegram через клиентский API (MTProto).
//
// Именно клиентский, а не Bot API: бот видит только чаты, куда его добавили,
// и личную переписку не отдаёт в принципе — это ограничение платформы, а не
// настройка. Личные сообщения доступны только под своим аккаунтом, тем же
// протоколом, на котором работают сами клиенты Telegram (решение №52).
//
// Плата за это — вход по номеру телефона и сессия в базе, которая даёт
// полный доступ к аккаунту. Приложение локальное, поэтому цена приемлемая,
// но обращаться с сессией нужно как с паролем.
package telegram

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/gotd/td/session"
	tgclient "github.com/gotd/td/telegram"
	"github.com/gotd/td/telegram/auth"
	"github.com/gotd/td/tg"

	"personalinbox/internal/exceptions"
	"personalinbox/internal/postgres"
	"personalinbox/internal/services/ingest"
)

const (
	// dialogsLimit — сколько диалогов просматриваем за один проход. Верхних
	// сорока хватает: ниже лежит то, что молчит месяцами.
	dialogsLimit = 40
	// historyLimit — сколько сообщений берём из одного диалога за проход.
	historyLimit = 30
	// importDays и importLimit — глубина первого импорта, как у Gmail
	// (решение №16).
	importDays  = 30
	importLimit = 200
	// overlap — на сколько отматываем назад от прошлой синхронизации: часы
	// клиента и сервера расходятся, а повтор отсеет дедупликация ingest.
	overlap = 2 * time.Minute
)

// Client — доступ к клиентскому API. Ключи берутся с my.telegram.org
// и передаются через окружение.
type Client struct {
	APIID   int
	APIHash string
}

// NewClient собирает клиента. Без ключей он не работает и честно об этом
// говорит при первой же попытке подключения.
func NewClient(apiID int, apiHash string) *Client {
	return &Client{APIID: apiID, APIHash: apiHash}
}

// Configured — заданы ли ключи API.
func (c *Client) Configured() bool {
	return c.APIID != 0 && c.APIHash != ""
}

// Pending — промежуточное состояние между «отправили код» и «ввели код».
// Живёт в Redis несколько минут: сессия здесь уже содержит ключ шифрования,
// поэтому хранить её дольше нужного незачем.
type Pending struct {
	Phone    string `json:"phone"`
	CodeHash string `json:"code_hash"`
	Session  string `json:"session"`
}

// Credentials — то, что лежит в connection.Credentials после входа.
type Credentials struct {
	Session string `json:"session"`
}

// ErrPasswordNeeded — у аккаунта включена двухфакторная защита, нужен пароль.
var ErrPasswordNeeded = errors.New("нужен пароль двухфакторной защиты")

// storageFrom восстанавливает хранилище сессии из строки. Пустая строка —
// новая сессия.
func storageFrom(encoded string) (*session.StorageMemory, error) {
	storage := &session.StorageMemory{}
	if encoded == "" {
		return storage, nil
	}
	raw, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return nil, fmt.Errorf("%w: сессия повреждена", exceptions.ErrTelegram)
	}
	if err := storage.StoreSession(context.Background(), raw); err != nil {
		return nil, fmt.Errorf("%w: сессия не читается", exceptions.ErrTelegram)
	}
	return storage, nil
}

func encodeSession(ctx context.Context, storage *session.StorageMemory) (string, error) {
	raw, err := storage.LoadSession(ctx)
	if err != nil {
		return "", fmt.Errorf("%w: сессию не сохранить: %v", exceptions.ErrTelegram, err)
	}
	return base64.StdEncoding.EncodeToString(raw), nil
}

// run поднимает соединение, выполняет работу и закрывает его. Постоянного
// подключения нет намеренно: синхронизация идёт раз в 5 минут, держать сокет
// между заходами незачем.
func (c *Client) run(ctx context.Context, storage *session.StorageMemory,
	work func(ctx context.Context, api *tg.Client, client *tgclient.Client) error) error {
	if !c.Configured() {
		return fmt.Errorf("%w: не заданы TELEGRAM_API_ID и TELEGRAM_API_HASH", exceptions.ErrTelegram)
	}
	client := tgclient.NewClient(c.APIID, c.APIHash, tgclient.Options{SessionStorage: storage})
	return client.Run(ctx, func(ctx context.Context) error {
		return work(ctx, client.API(), client)
	})
}

// SendCode просит Telegram прислать код подтверждения на номер. Возвращает
// состояние, которое нужно передать в SignIn.
func (c *Client) SendCode(ctx context.Context, phone string) (Pending, error) {
	phone = strings.TrimSpace(phone)
	storage := &session.StorageMemory{}
	var pending Pending

	err := c.run(ctx, storage, func(ctx context.Context, _ *tg.Client, client *tgclient.Client) error {
		sent, err := client.Auth().SendCode(ctx, phone, auth.SendCodeOptions{})
		if err != nil {
			return fmt.Errorf("%w: %v", exceptions.ErrTelegram, err)
		}
		code, ok := sent.(*tg.AuthSentCode)
		if !ok {
			return fmt.Errorf("%w: Telegram не прислал код", exceptions.ErrTelegram)
		}
		pending.CodeHash = code.PhoneCodeHash
		return nil
	})
	if err != nil {
		return Pending{}, err
	}

	encoded, err := encodeSession(ctx, storage)
	if err != nil {
		return Pending{}, err
	}
	pending.Phone = phone
	pending.Session = encoded
	return pending, nil
}

// SignIn завершает вход. Пустой пароль допустим: он нужен только аккаунтам
// с двухфакторной защитой, и тогда возвращается ErrPasswordNeeded.
// Второе возвращаемое значение — подпись аккаунта для карточки источника.
func (c *Client) SignIn(ctx context.Context, pending Pending, code, password string) (string, string, error) {
	storage, err := storageFrom(pending.Session)
	if err != nil {
		return "", "", err
	}

	var account string
	err = c.run(ctx, storage, func(ctx context.Context, _ *tg.Client, client *tgclient.Client) error {
		_, err := client.Auth().SignIn(ctx, pending.Phone, strings.TrimSpace(code), pending.CodeHash)
		if errors.Is(err, auth.ErrPasswordAuthNeeded) {
			if password == "" {
				return ErrPasswordNeeded
			}
			if _, err := client.Auth().Password(ctx, password); err != nil {
				return fmt.Errorf("%w: пароль не подошёл", exceptions.ErrTelegram)
			}
		} else if err != nil {
			return fmt.Errorf("%w: %v", exceptions.ErrTelegram, err)
		}

		self, err := client.Self(ctx)
		if err != nil {
			return fmt.Errorf("%w: не удалось прочитать профиль", exceptions.ErrTelegram)
		}
		account = accountTitle(self)
		return nil
	})
	if err != nil {
		return "", "", err
	}

	encoded, err := encodeSession(ctx, storage)
	if err != nil {
		return "", "", err
	}
	return encoded, account, nil
}

// accountTitle — подпись аккаунта на карточке источника.
func accountTitle(self *tg.User) string {
	if self == nil {
		return "аккаунт Telegram"
	}
	if self.Username != "" {
		return "@" + self.Username
	}
	name := strings.TrimSpace(self.FirstName + " " + self.LastName)
	if name != "" {
		return name
	}
	if self.Phone != "" {
		return "+" + strings.TrimPrefix(self.Phone, "+")
	}
	return "аккаунт Telegram"
}

// Sync забирает новые сообщения из диалогов. Возвращает число сохранённых.
func (c *Client) Sync(ingestor *ingest.Ingestor, connection *postgres.Connection) (int, error) {
	var credentials Credentials
	if connection.Credentials != "" {
		_ = json.Unmarshal([]byte(connection.Credentials), &credentials)
	}
	if credentials.Session == "" {
		connection.State = "reauth"
		return 0, ingestor.DB.SaveConnection(connection)
	}

	storage, err := storageFrom(credentials.Session)
	if err != nil {
		connection.State = "reauth"
		return 0, ingestor.DB.SaveConnection(connection)
	}

	// Первое подключение забирает историю за месяц, дальше — только то,
	// что появилось с прошлого раза.
	since := postgres.UTCNow().AddDate(0, 0, -importDays)
	limit := importLimit
	if connection.LastSyncAt != nil {
		since = connection.LastSyncAt.Add(-overlap)
		limit = historyLimit * dialogsLimit
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	saved := 0
	err = c.run(ctx, storage, func(ctx context.Context, api *tg.Client, _ *tgclient.Client) error {
		count, err := c.fetch(ctx, api, ingestor, connection, since, limit)
		saved = count
		return err
	})
	if err != nil {
		// Сессия отозвана — просим войти заново; сетевой сбой просто ждёт
		// следующего цикла.
		log.Printf("синхронизация telegram: %v", err)
		if sessionRevoked(err) {
			connection.State = "reauth"
			return saved, ingestor.DB.SaveConnection(connection)
		}
		return saved, nil
	}

	now := postgres.UTCNow()
	connection.LastSyncAt = &now
	connection.State = "active"
	return saved, ingestor.DB.SaveConnection(connection)
}

// sessionRevoked — отличает «войдите заново» от временного сбоя.
func sessionRevoked(err error) bool {
	text := strings.ToUpper(err.Error())
	for _, marker := range []string{"AUTH_KEY_UNREGISTERED", "SESSION_REVOKED",
		"SESSION_EXPIRED", "USER_DEACTIVATED", "AUTH_KEY_DUPLICATED"} {
		if strings.Contains(text, marker) {
			return true
		}
	}
	return false
}

// fetch проходит по диалогам и складывает новые сообщения.
func (c *Client) fetch(ctx context.Context, api *tg.Client, ingestor *ingest.Ingestor,
	connection *postgres.Connection, since time.Time, limit int) (int, error) {
	dialogs, err := api.MessagesGetDialogs(ctx, &tg.MessagesGetDialogsRequest{
		OffsetPeer: &tg.InputPeerEmpty{},
		Limit:      dialogsLimit,
	})
	if err != nil {
		return 0, fmt.Errorf("%w: список диалогов: %v", exceptions.ErrTelegram, err)
	}

	peers := collectPeers(dialogs)
	saved := 0
	for _, key := range peers.order {
		if saved >= limit {
			break
		}
		source := peers.byKey[key]
		messages, err := api.MessagesGetHistory(ctx, &tg.MessagesGetHistoryRequest{
			Peer:  source.input,
			Limit: historyLimit,
		})
		if err != nil {
			// Один недоступный чат не должен ронять всю синхронизацию.
			log.Printf("telegram: история чата %s: %v", key, err)
			continue
		}
		for _, item := range messagesOf(messages) {
			incoming, ok := incomingFrom(item, source)
			if !ok || incoming.ReceivedAt.Before(since) {
				continue
			}
			message, err := ingestor.Store(connection, incoming)
			if err != nil {
				return saved, err
			}
			if message != nil {
				saved++
			}
		}
	}
	return saved, nil
}
