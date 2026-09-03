// Package gmail — источник Gmail через Google OAuth 2.0 и Gmail API,
// доступ только на чтение.
//
// Первый импорт — письма за 30 дней, не больше 200 (решение №16). Дальше
// инкрементально по historyId. Истёкший или отозванный токен переводит
// подключение в reauth, ранее полученные сообщения остаются в ленте.
package gmail

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/mail"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"

	"personalinbox/internal/config"
	"personalinbox/internal/ingest"
	"personalinbox/internal/store"
)

// ErrGmail — Google не отвечает или отказал в доступе.
var ErrGmail = errors.New("gmail")

const (
	scope         = "https://www.googleapis.com/auth/gmail.readonly"
	messageURL    = "https://mail.google.com/mail/u/0/#inbox/%s"
	pageSize      = 100
	defaultAuth   = "https://accounts.google.com/o/oauth2/auth"
	defaultToken  = "https://oauth2.googleapis.com/token"
	defaultAPI    = "https://gmail.googleapis.com/gmail/v1"
	requestTimout = 30 * time.Second
)

// Client — работа с Google по HTTP: SDK ради шести запросов не нужен.
// Адреса вынесены в поля, чтобы тесты подставляли свой сервер.
type Client struct {
	ClientID     string
	ClientSecret string
	RedirectURI  string
	AuthEndpoint string
	TokenURL     string
	APIBase      string
	HTTP         *http.Client
}

// NewClient собирает клиента из настроек.
func NewClient(cfg config.Config) *Client {
	return &Client{
		ClientID:     cfg.GoogleClientID,
		ClientSecret: cfg.GoogleClientSecret,
		RedirectURI:  cfg.GoogleRedirectURI,
		AuthEndpoint: defaultAuth,
		TokenURL:     defaultToken,
		APIBase:      defaultAPI,
		HTTP:         &http.Client{Timeout: requestTimout},
	}
}

func (c *Client) client() *http.Client {
	if c.HTTP == nil {
		return &http.Client{Timeout: requestTimout}
	}
	return c.HTTP
}

func (c *Client) checkCredentials() error {
	if c.ClientID == "" || c.ClientSecret == "" {
		return fmt.Errorf("%w: GOOGLE_CLIENT_ID и GOOGLE_CLIENT_SECRET не заданы", ErrGmail)
	}
	return nil
}

// AuthURL — адрес согласия Google. access_type=offline нужен ради refresh_token.
func (c *Client) AuthURL(state string) (string, error) {
	if err := c.checkCredentials(); err != nil {
		return "", err
	}
	endpoint := c.AuthEndpoint
	if endpoint == "" {
		endpoint = defaultAuth
	}
	params := url.Values{
		"client_id":              {c.ClientID},
		"redirect_uri":           {c.RedirectURI},
		"response_type":          {"code"},
		"scope":                  {scope},
		"access_type":            {"offline"},
		"include_granted_scopes": {"true"},
		"prompt":                 {"consent"},
		"state":                  {state},
	}
	return endpoint + "?" + params.Encode(), nil
}

// ExchangeCode меняет код на токены. Возвращает (email аккаунта, refresh_token).
func (c *Client) ExchangeCode(code string) (string, string, error) {
	if err := c.checkCredentials(); err != nil {
		return "", "", err
	}
	tokens, err := c.postToken(url.Values{
		"code":          {code},
		"client_id":     {c.ClientID},
		"client_secret": {c.ClientSecret},
		"redirect_uri":  {c.RedirectURI},
		"grant_type":    {"authorization_code"},
	})
	if err != nil {
		return "", "", fmt.Errorf("%w: Google отказал в обмене кода: %v", ErrGmail, err)
	}
	if tokens.RefreshToken == "" {
		return "", "", fmt.Errorf(
			"%w: Google не выдал refresh_token — переподключите доступ", ErrGmail)
	}
	email, err := c.profileEmail(tokens.AccessToken)
	if err != nil {
		return "", "", err
	}
	return email, tokens.RefreshToken, nil
}

type tokenResponse struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
}

func (c *Client) postToken(form url.Values) (tokenResponse, error) {
	endpoint := c.TokenURL
	if endpoint == "" {
		endpoint = defaultToken
	}
	response, err := c.client().PostForm(endpoint, form)
	if err != nil {
		return tokenResponse{}, err
	}
	defer response.Body.Close()

	raw, err := io.ReadAll(response.Body)
	if err != nil {
		return tokenResponse{}, err
	}
	if response.StatusCode != http.StatusOK {
		return tokenResponse{}, fmt.Errorf("%d %s", response.StatusCode, strings.TrimSpace(string(raw)))
	}
	var tokens tokenResponse
	if err := json.Unmarshal(raw, &tokens); err != nil {
		return tokenResponse{}, err
	}
	return tokens, nil
}

// accessToken обновляет короткоживущий токен по refresh_token.
func (c *Client) accessToken(refreshToken string) (string, error) {
	if err := c.checkCredentials(); err != nil {
		return "", err
	}
	tokens, err := c.postToken(url.Values{
		"refresh_token": {refreshToken},
		"client_id":     {c.ClientID},
		"client_secret": {c.ClientSecret},
		"grant_type":    {"refresh_token"},
	})
	if err != nil {
		return "", fmt.Errorf("%w: обновление токена: %v", ErrGmail, err)
	}
	if tokens.AccessToken == "" {
		return "", fmt.Errorf("%w: Google не выдал access_token", ErrGmail)
	}
	return tokens.AccessToken, nil
}

// get — запрос к Gmail API с токеном доступа.
func (c *Client) get(accessToken, path string, params url.Values) (json.RawMessage, error) {
	base := c.APIBase
	if base == "" {
		base = defaultAPI
	}
	endpoint := base + path
	if len(params) > 0 {
		endpoint += "?" + params.Encode()
	}
	request, err := http.NewRequest(http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrGmail, err)
	}
	request.Header.Set("Authorization", "Bearer "+accessToken)

	response, err := c.client().Do(request)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrGmail, err)
	}
	defer response.Body.Close()

	raw, err := io.ReadAll(response.Body)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrGmail, err)
	}
	if response.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("%w: %d %s", ErrGmail, response.StatusCode,
			strings.TrimSpace(string(raw)))
	}
	return raw, nil
}

func (c *Client) profileEmail(accessToken string) (string, error) {
	raw, err := c.get(accessToken, "/users/me/profile", nil)
	if err != nil {
		return "", err
	}
	var profile struct {
		EmailAddress string `json:"emailAddress"`
	}
	if err := json.Unmarshal(raw, &profile); err != nil {
		return "", fmt.Errorf("%w: профиль не разобрать", ErrGmail)
	}
	return profile.EmailAddress, nil
}

func (c *Client) profileHistoryID(accessToken string) string {
	raw, err := c.get(accessToken, "/users/me/profile", nil)
	if err != nil {
		return ""
	}
	var profile struct {
		HistoryID json.Number `json:"historyId"`
	}
	if err := json.Unmarshal(raw, &profile); err != nil {
		return ""
	}
	return profile.HistoryID.String()
}

// --- разбор письма ---------------------------------------------------------

type payload struct {
	MimeType string `json:"mimeType"`
	Headers  []struct {
		Name  string `json:"name"`
		Value string `json:"value"`
	} `json:"headers"`
	Body struct {
		Data string `json:"data"`
	} `json:"body"`
	Parts []payload `json:"parts"`
}

type rawMessage struct {
	ID           string  `json:"id"`
	Snippet      string  `json:"snippet"`
	InternalDate string  `json:"internalDate"`
	Payload      payload `json:"payload"`
}

func header(source payload, name string) string {
	for _, item := range source.Headers {
		if strings.EqualFold(item.Name, name) {
			return item.Value
		}
	}
	return ""
}

func decode(data string) string {
	decoded, err := base64.URLEncoding.WithPadding(base64.NoPadding).
		DecodeString(strings.TrimRight(data, "="))
	if err != nil {
		return ""
	}
	return string(decoded)
}

var (
	scriptRe = regexp.MustCompile(`(?is)<(script|style)[^>]*>.*?</(script|style)>`)
	tagRe    = regexp.MustCompile(`(?s)<[^>]+>`)
	spaceRe  = regexp.MustCompile(`\s+`)
)

func stripHTML(html string) string {
	text := scriptRe.ReplaceAllString(html, " ")
	text = tagRe.ReplaceAllString(text, " ")
	return strings.TrimSpace(spaceRe.ReplaceAllString(text, " "))
}

// bodyText — текстовое содержимое письма. Сначала ищем text/plain по всему
// дереву частей и только потом соглашаемся на HTML: в multipart/alternative
// части идут в произвольном порядке.
func bodyText(source payload) string {
	if text := partByType(source, "text/plain"); text != "" {
		return decode(text)
	}
	if text := partByType(source, "text/html"); text != "" {
		return stripHTML(decode(text))
	}
	return ""
}

// partByType — данные первой части нужного типа, обходя вложенные multipart.
func partByType(source payload, mimeType string) string {
	if source.Body.Data != "" && source.MimeType == mimeType {
		return source.Body.Data
	}
	for _, part := range source.Parts {
		if data := partByType(part, mimeType); data != "" {
			return data
		}
	}
	return ""
}

func receivedAt(raw rawMessage) time.Time {
	if raw.InternalDate != "" {
		if millis, err := strconv.ParseInt(raw.InternalDate, 10, 64); err == nil {
			return time.UnixMilli(millis).UTC()
		}
	}
	if value := header(raw.Payload, "Date"); value != "" {
		if parsed, err := mail.ParseDate(value); err == nil {
			return parsed.UTC()
		}
	}
	return store.UTCNow()
}

// ToIncoming переводит письмо Gmail в общий формат приёма.
func ToIncoming(raw rawMessage) ingest.Incoming {
	senderName, senderAddr := "", ""
	if from := header(raw.Payload, "From"); from != "" {
		if address, err := mail.ParseAddress(from); err == nil {
			senderName, senderAddr = address.Name, address.Address
		} else {
			senderAddr = strings.TrimSpace(from)
		}
	}
	if senderName == "" {
		senderName = senderAddr
	}
	if senderName == "" {
		senderName = "Без отправителя"
	}
	subject := header(raw.Payload, "Subject")
	if subject == "" {
		subject = "(без темы)"
	}
	body := bodyText(raw.Payload)
	if body == "" {
		body = raw.Snippet
	}
	return ingest.Incoming{
		ExternalID:  raw.ID,
		SenderName:  senderName,
		SenderAddr:  senderAddr,
		Subject:     subject,
		Body:        body,
		ReceivedAt:  receivedAt(raw),
		ExternalURL: fmt.Sprintf(messageURL, raw.ID),
	}
}

// --- синхронизация ---------------------------------------------------------

// IsAuthError — истёкший или отозванный доступ: подключение уходит в reauth.
func IsAuthError(err error) bool {
	if err == nil {
		return false
	}
	text := strings.ToLower(err.Error())
	for _, marker := range []string{"invalid_grant", "unauthorized", "401", "403"} {
		if strings.Contains(text, marker) {
			return true
		}
	}
	return false
}

// isStaleHistory — Google хранит историю ограниченное время: протухший
// historyId не ошибка, а повод импортировать заново.
func isStaleHistory(err error) bool {
	if err == nil {
		return false
	}
	text := strings.ToLower(err.Error())
	return strings.Contains(text, "404") ||
		strings.Contains(text, "not found") ||
		strings.Contains(text, "starthistoryid")
}

// Sync синхронизирует одно подключение. Возвращает число новых сообщений.
func (c *Client) Sync(ingestor *ingest.Ingestor, connection *store.Connection) (int, error) {
	var credentials struct {
		RefreshToken string `json:"refresh_token"`
	}
	if connection.Credentials != "" {
		_ = json.Unmarshal([]byte(connection.Credentials), &credentials)
	}
	if credentials.RefreshToken == "" {
		connection.State = "reauth"
		return 0, ingestor.DB.SaveConnection(connection)
	}

	saved, err := c.sync(ingestor, connection, credentials.RefreshToken)
	if err != nil {
		log.Printf("синхронизация gmail: %v", err)
		if IsAuthError(err) {
			connection.State = "reauth"
			return 0, ingestor.DB.SaveConnection(connection)
		}
		return 0, nil
	}
	return saved, nil
}

func (c *Client) sync(ingestor *ingest.Ingestor, connection *store.Connection, refreshToken string) (int, error) {
	accessToken, err := c.accessToken(refreshToken)
	if err != nil {
		return 0, err
	}
	ids, cursor, err := c.collectIDs(accessToken, connection.SyncCursor)
	if err != nil {
		return 0, err
	}

	saved := 0
	for _, id := range ids {
		raw, err := c.get(accessToken, "/users/me/messages/"+id, url.Values{"format": {"full"}})
		if err != nil {
			return saved, err
		}
		var letter rawMessage
		if err := json.Unmarshal(raw, &letter); err != nil {
			continue
		}
		message, err := ingestor.Store(connection, ToIncoming(letter))
		if err != nil {
			return saved, err
		}
		if message != nil {
			saved++
		}
	}

	if cursor == "" {
		cursor = c.profileHistoryID(accessToken)
	}
	if cursor != "" {
		connection.SyncCursor = cursor
	}
	now := store.UTCNow()
	connection.LastSyncAt = &now
	connection.State = "active"
	return saved, ingestor.DB.SaveConnection(connection)
}

// collectIDs решает, что забирать: инкремент по historyId либо первый импорт
// за 30 дней.
func (c *Client) collectIDs(accessToken, syncCursor string) ([]string, string, error) {
	if syncCursor == "" {
		ids, err := c.initialIDs(accessToken)
		return ids, "", err
	}
	ids, cursor, err := c.incrementalIDs(accessToken, syncCursor)
	if err == nil {
		return ids, cursor, nil
	}
	if !isStaleHistory(err) {
		return nil, "", err
	}
	log.Print("historyId устарел, импортируем письма заново")
	ids, err = c.initialIDs(accessToken)
	return ids, "", err
}

// initialIDs — первый импорт: письма за 30 дней, максимум 200, новые первыми.
func (c *Client) initialIDs(accessToken string) ([]string, error) {
	after := time.Now().UTC().AddDate(0, 0, -config.GmailImportDays).Format("2006/01/02")
	var ids []string
	pageToken := ""
	for len(ids) < config.GmailImportLimit {
		limit := config.GmailImportLimit - len(ids)
		if limit > pageSize {
			limit = pageSize
		}
		params := url.Values{
			"q":          {"after:" + after},
			"maxResults": {strconv.Itoa(limit)},
		}
		if pageToken != "" {
			params.Set("pageToken", pageToken)
		}
		raw, err := c.get(accessToken, "/users/me/messages", params)
		if err != nil {
			return nil, err
		}
		var page struct {
			Messages []struct {
				ID string `json:"id"`
			} `json:"messages"`
			NextPageToken string `json:"nextPageToken"`
		}
		if err := json.Unmarshal(raw, &page); err != nil {
			return nil, fmt.Errorf("%w: список писем не разобрать", ErrGmail)
		}
		for _, item := range page.Messages {
			ids = append(ids, item.ID)
		}
		pageToken = page.NextPageToken
		if pageToken == "" {
			break
		}
	}
	if len(ids) > config.GmailImportLimit {
		ids = ids[:config.GmailImportLimit]
	}
	return ids, nil
}

// incrementalIDs — что добавилось после сохранённого historyId.
func (c *Client) incrementalIDs(accessToken, startHistoryID string) ([]string, string, error) {
	var ids []string
	seen := map[string]bool{}
	cursor := startHistoryID
	pageToken := ""
	for {
		params := url.Values{
			"startHistoryId": {startHistoryID},
			"historyTypes":   {"messageAdded"},
		}
		if pageToken != "" {
			params.Set("pageToken", pageToken)
		}
		raw, err := c.get(accessToken, "/users/me/history", params)
		if err != nil {
			return nil, "", err
		}
		var page struct {
			History []struct {
				MessagesAdded []struct {
					Message struct {
						ID string `json:"id"`
					} `json:"message"`
				} `json:"messagesAdded"`
			} `json:"history"`
			HistoryID     json.Number `json:"historyId"`
			NextPageToken string      `json:"nextPageToken"`
		}
		if err := json.Unmarshal(raw, &page); err != nil {
			return nil, "", fmt.Errorf("%w: историю не разобрать", ErrGmail)
		}
		for _, record := range page.History {
			for _, added := range record.MessagesAdded {
				id := added.Message.ID
				if id != "" && !seen[id] {
					seen[id] = true
					ids = append(ids, id)
				}
			}
		}
		if value := page.HistoryID.String(); value != "" {
			cursor = value
		}
		pageToken = page.NextPageToken
		if pageToken == "" {
			break
		}
	}
	return ids, cursor, nil
}
