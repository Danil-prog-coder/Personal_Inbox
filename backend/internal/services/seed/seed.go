// Package seed — демо-данные из референса: 19 сообщений ленты и 3 сообщения
// очереди живой демонстрации. Перенесены дословно (docs/03-data-model.md, п. 7) —
// набор специально разнородный, на нём проверяются крайние случаи вёрстки.
// Не сокращать и не заменять.
package seed

import (
	"encoding/json"
	"errors"
	"personalinbox/internal/exceptions"
	"strconv"
	"strings"
	"time"

	"personalinbox/internal/events"
	"personalinbox/internal/postgres"
	"personalinbox/internal/schemas"
)

// DemoCriteria — критерии важности, с которыми заливается демо-лента.
const DemoCriteria = "Важны письма от клиентов Northline, всё про договоры и сроки, " +
	"сообщения от команды с блокерами. Рассылки и уведомления сервисов — неважно."

// Accounts — то, что показано в UI на карточках источников.
var Accounts = map[string]string{"gmail": "me@northline.io", "telegram": "@maxorlov"}

// States — в референсе Telegram показан как требующий повторной авторизации:
// состояние reauth нужно, чтобы на демо-данных было видно предупреждение.
var States = map[string]string{"gmail": "active", "telegram": "reauth"}

var srcToKind = map[string]string{"gmail": "gmail", "tg": "telegram"}

// Item — сообщение демо-ленты в том виде, в каком оно записано в референсе.
type Item struct {
	ID       int
	Src      string
	From     string
	Addr     string
	Time     string
	Subj     string
	Text     string
	Level    string
	Action   bool
	Reply    bool
	Unread   bool
	Cat      string
	Deadline string
	Sum      string
}

// Verdict — оценка, которую «выдаёт модель» в живой демонстрации.
type Verdict struct {
	Level    string
	Action   bool
	Reply    bool
	Cat      string
	Deadline string
	Sum      string
}

// LiveItem — сообщение очереди живой демонстрации.
type LiveItem struct {
	Src  string
	From string
	Addr string
	Subj string
	Text string
	Res  Verdict
}

// Messages — 19 сообщений ленты: 11 Gmail и 8 Telegram.
var Messages = []Item{
	{ID: 1, Src: "gmail", From: "Анна Ковалёва", Addr: "a.kovaleva@northline.io", Time: "09:41",
		Subj:  "Договор Northline — правки до конца дня",
		Text:  "Юристы вернули версию с комментариями. Нужна ваша реакция по пунктам 4.2 и 7 до 18:00, иначе подписание уезжает на следующую неделю.",
		Level: "CRITICAL", Action: true, Reply: true, Unread: true,
		Cat: "Юридическое", Deadline: "Сегодня, 18:00",
		Sum: "Правки к договору требуют вашего решения по двум пунктам до 18:00 сегодня."},
	{ID: 2, Src: "gmail", From: "Налоговая служба", Addr: "noreply@nalog.gov", Time: "08:52",
		Subj:  "Уведомление: срок подачи декларации",
		Text:  "Напоминаем о необходимости подать декларацию за отчётный период до 30 сентября. При нарушении срока начисляется штраф.",
		Level: "HIGH", Action: true, Reply: false, Unread: true,
		Cat: "Документы", Deadline: "30 сентября",
		Sum: "Декларацию нужно подать до 30 сентября, иначе штраф."},
	{ID: 3, Src: "gmail", From: "Stripe", Addr: "no-reply@stripe.com", Time: "08:30",
		Subj:  "Платёж на $1 240.00 прошёл успешно",
		Text:  "Ваш ежемесячный платёж по подписке Northline Pro обработан. Квитанция во вложении, следующее списание 2 октября.",
		Level: "NORMAL", Action: false, Reply: false, Unread: false,
		Cat: "Финансы", Deadline: "",
		Sum: "Ежемесячный платёж $1 240.00 успешно прошёл."},
	{ID: 4, Src: "gmail", From: "Ольга Тимченко", Addr: "o.timchenko@hr-lab.ru", Time: "Вчера, 19:04",
		Subj:  "Оффер senior-разработчику — нужна ваша виза",
		Text:  "Финальный кандидат ждёт подтверждения вилки: 380–420 тыс. Он держит второй оффер до вторника, поэтому решение хорошо бы принять завтра.",
		Level: "HIGH", Action: true, Reply: true, Unread: true,
		Cat: "Найм", Deadline: "Вторник",
		Sum: "Требуется согласование вилки по офферу; кандидат держит второй оффер до вторника."},
	{ID: 5, Src: "gmail", From: "AWS Billing", Addr: "aws-billing@amazon.com", Time: "Вчера, 16:20",
		Subj:  "Прогноз счёта превышен на 42%",
		Text:  "Текущее потребление по аккаунту 4471-9902 превышает бюджет месяца. Основной рост — RDS и исходящий трафик CloudFront.",
		Level: "HIGH", Action: true, Reply: false, Unread: false,
		Cat: "Инфраструктура", Deadline: "",
		Sum: "Расход по AWS выше бюджета на 42%, рост в RDS и CloudFront."},
	{ID: 6, Src: "gmail", From: "Мария Соколова", Addr: "m.sokolova@northline.io", Time: "Вчера, 14:10",
		Subj:  "Макеты онбординга — три варианта",
		Text:  "Закинула в папку три направления: минимальное, с прогрессом и с примерами. Посмотри, когда будет минутка — не срочно, до ревью в пятницу.",
		Level: "NORMAL", Action: false, Reply: true, Unread: false,
		Cat: "Дизайн", Deadline: "Пятница",
		Sum: "Три варианта макетов онбординга готовы, ревью в пятницу."},
	{ID: 7, Src: "gmail", From: "Сбербанк Бизнес", Addr: "info@sberbank.ru", Time: "Вчера, 11:37",
		Subj:  "Выписка по счёту за август",
		Text:  "Выписка сформирована и доступна в личном кабинете. Обороты за месяц — 4,8 млн ₽.",
		Level: "NORMAL", Action: false, Reply: false, Unread: false,
		Cat: "Финансы", Deadline: "",
		Sum: "Августовская выписка готова, обороты 4,8 млн ₽."},
	{ID: 8, Src: "gmail", From: "Product Weekly", Addr: "digest@productweekly.co", Time: "Пн",
		Subj:  "Выпуск 214: что нового в AI-инструментах",
		Text:  "Еженедельная подборка: девять разборов, интервью с командой Linear и колонка про приоритизацию бэклога.",
		Level: "LOW", Action: false, Reply: false, Unread: false,
		Cat: "Рассылка", Deadline: "",
		Sum: "Регулярный дайджест, действий не требует."},
	{ID: 9, Src: "gmail", From: "Figma", Addr: "updates@figma.com", Time: "Пн",
		Subj:  "Ваш план Organization продлён на год",
		Text:  "Спасибо, что остаётесь с нами. Счёт и детали тарифа доступны в настройках биллинга.",
		Level: "LOW", Action: false, Reply: false, Unread: false,
		Cat: "Сервисы", Deadline: "",
		Sum: "Подписка Figma продлена автоматически."},
	{ID: 10, Src: "gmail", From: "Кирилл Ушаков", Addr: "k.ushakov@vendorlab.io", Time: "Вс",
		Subj:  "Коммерческое предложение по интеграции",
		Text:  "Подготовили расчёт под ваш объём: 2,1 млн ₽ за внедрение и 180 тыс. ₽ в месяц. Готовы обсудить на звонке.",
		Level: "NORMAL", Action: false, Reply: true, Unread: false,
		Cat: "Партнёры", Deadline: "",
		Sum: "Вендор прислал расчёт: 2,1 млн ₽ внедрение, 180 тыс. ₽ в месяц."},
	{ID: 11, Src: "gmail", From: "Google Workspace", Addr: "security@google.com", Time: "Сб",
		Subj:  "Новый вход с устройства MacBook Pro",
		Text:  "Обнаружен вход в аккаунт из Москвы. Если это были вы, действий не требуется.",
		Level: "LOW", Action: false, Reply: false, Unread: false,
		Cat: "Безопасность", Deadline: "",
		Sum: "Уведомление о входе с нового устройства."},
	{ID: 12, Src: "tg", From: "Инвесторы · Seed round", Addr: "групповой чат, 9 участников", Time: "09:22",
		Subj:  "Питч-дек — шесть вопросов от фонда",
		Text:  "Прислали вопросы по юнит-экономике: CAC по каналам, retention 6+ месяцев и структура косвенных расходов. Ждут ответ к среде.",
		Level: "CRITICAL", Action: true, Reply: true, Unread: true,
		Cat: "Инвестиции", Deadline: "Среда",
		Sum: "Фонд ждёт ответы по юнит-экономике к среде."},
	{ID: 13, Src: "tg", From: "Дима · Продукт", Addr: "@dmitry_pm", Time: "09:12",
		Subj:  "Релиз 2.4 — блокер на бэкенде",
		Text:  "Ребята нашли гонку в очереди задач: при пиковой нагрузке часть событий обрабатывается дважды. Предлагаю сдвинуть релиз на завтра утро. Ок?",
		Level: "HIGH", Action: true, Reply: true, Unread: true,
		Cat: "Разработка", Deadline: "Сегодня",
		Sum: "Просят подтвердить перенос релиза 2.4 на завтра из-за блокера."},
	{ID: 14, Src: "tg", From: "Northline · дежурство", Addr: "@northline_alerts", Time: "07:48",
		Subj:  "API p95 вырос до 1,8 с",
		Text:  "Алерт по латентности держится 20 минут. Дежурный смотрит, похоже на медленный запрос в отчётах.",
		Level: "HIGH", Action: false, Reply: false, Unread: true,
		Cat: "Инфраструктура", Deadline: "",
		Sum: "Латентность API выросла, дежурный уже разбирается."},
	{ID: 15, Src: "tg", From: "Саша Ким", Addr: "@sasha_k", Time: "Вчера, 21:30",
		Subj:  "Кофе на неделе?",
		Text:  "Я в городе до пятницы, давай пересечёмся — расскажу, чем закончилась история с их раундом.",
		Level: "NORMAL", Action: false, Reply: true, Unread: false,
		Cat: "Личное", Deadline: "Пятница",
		Sum: "Предлагает встретиться до пятницы."},
	{ID: 16, Src: "tg", From: "Лена · Бухгалтерия", Addr: "@lena_acc", Time: "Вчера, 17:02",
		Subj:  "Нужны закрывающие по трём подрядчикам",
		Text:  "Без актов не закрою квартал. Скинь, пожалуйста, до конца недели — иначе перенесётся в следующий период.",
		Level: "HIGH", Action: true, Reply: true, Unread: false,
		Cat: "Финансы", Deadline: "Пятница",
		Sum: "Нужны закрывающие документы по трём подрядчикам до конца недели."},
	{ID: 17, Src: "tg", From: "Дизайн-ревью", Addr: "групповой чат, 5 участников", Time: "Пн",
		Subj:  "Обсудили новую навигацию",
		Text:  "Сошлись на варианте с боковой панелью. Максим, нужен твой финальный голос перед разработкой.",
		Level: "NORMAL", Action: true, Reply: true, Unread: false,
		Cat: "Дизайн", Deadline: "",
		Sum: "Команда выбрала боковую навигацию, ждут ваш голос."},
	{ID: 18, Src: "tg", From: "Спортзал «Форма»", Addr: "@formagym", Time: "Сб",
		Subj:  "Абонемент заканчивается через 5 дней",
		Text:  "Продлите до 7 сентября, чтобы сохранить текущую цену — с октября тарифы вырастут.",
		Level: "LOW", Action: false, Reply: false, Unread: false,
		Cat: "Личное", Deadline: "Через 5 дней",
		Sum: "Абонемент истекает через 5 дней."},
	{ID: 19, Src: "tg", From: "Доставка «Самокат»", Addr: "@samokat_bot", Time: "Сб",
		Subj:  "Заказ доставлен",
		Text:  "Курьер оставил заказ у двери. Спасибо, что выбираете нас!",
		Level: "LOW", Action: false, Reply: false, Unread: false,
		Cat: "Сервисы", Deadline: "",
		Sum: "Заказ доставлен, действий не требуется."},
}

// LiveQueue — три «новых» сообщения: на них тестируется появление карточек
// в реальном времени.
var LiveQueue = []LiveItem{
	{Src: "gmail", From: "Виктор Лебедев", Addr: "v.lebedev@northline.io",
		Subj: "Re: Договор Northline — согласовал п. 7",
		Text: "Принял вашу формулировку по седьмому пункту, остаётся 4.2. Если пришлёте до вечера, подпишем сегодня.",
		Res: Verdict{Level: "CRITICAL", Action: true, Reply: true, Cat: "Юридическое",
			Deadline: "Сегодня, 18:00",
			Sum:      "Пункт 7 согласован, остаётся решение по 4.2 — подпись сегодня."}},
	{Src: "tg", From: "Дима · Продукт", Addr: "@dmitry_pm",
		Subj: "Фикс блокера в мастере",
		Text: "Гонку в очереди починили, тесты зелёные. Возвращаем релиз на сегодняшний вечер?",
		Res: Verdict{Level: "HIGH", Action: true, Reply: true, Cat: "Разработка",
			Deadline: "Сегодня",
			Sum:      "Блокер починен, спрашивают про возврат релиза на вечер."}},
	{Src: "gmail", From: "Notion", Addr: "team@notion.so",
		Subj: "Вас упомянули в «Q4 планирование»",
		Text: "Мария Соколова оставила комментарий с вашим упоминанием в разделе «Ресурсы дизайна».",
		Res: Verdict{Level: "LOW", Action: false, Reply: false, Cat: "Сервисы",
			Deadline: "",
			Sum:      "Упоминание в документе, действий не требует."}},
}

var weekdays = map[string]int{"Пн": 0, "Вт": 1, "Ср": 2, "Чт": 3, "Пт": 4, "Сб": 5, "Вс": 6}

// pyWeekday — день недели, где понедельник равен нулю: так считает референс.
func pyWeekday(moment time.Time) int {
	return (int(moment.Weekday()) + 6) % 7
}

func atNoon(day time.Time) time.Time {
	return time.Date(day.Year(), day.Month(), day.Day(), 12, 0, 0, 0, time.UTC)
}

// ParseTime переводит время из референса («09:41», «Вчера, 19:04», «Пн»)
// в реальную метку. В БД хранится момент, строка вычисляется на фронте
// (docs/03-data-model.md).
func ParseTime(spec string, now time.Time) time.Time {
	spec = strings.TrimSpace(spec)
	if target, ok := weekdays[spec]; ok {
		delta := (pyWeekday(now) - target + 7) % 7
		if delta == 0 {
			delta = 7
		}
		return atNoon(now.AddDate(0, 0, -delta))
	}

	day := now
	if strings.HasPrefix(spec, "Вчера") {
		day = now.AddDate(0, 0, -1)
		if _, rest, ok := strings.Cut(spec, ","); ok {
			spec = strings.TrimSpace(rest)
		} else {
			spec = "12:00"
		}
	}
	hourText, minuteText, ok := strings.Cut(spec, ":")
	if !ok {
		return atNoon(day)
	}
	hour, errHour := strconv.Atoi(strings.TrimSpace(hourText))
	minute, errMinute := strconv.Atoi(strings.TrimSpace(minuteText))
	if errHour != nil || errMinute != nil {
		return atNoon(day)
	}
	return time.Date(day.Year(), day.Month(), day.Day(), hour, minute, 0, 0, time.UTC)
}

// externalURL — у группового чата без @username прямой ссылки нет,
// кнопка не показывается.
func externalURL(kind, addr, externalID string) string {
	if kind == "gmail" {
		return "https://mail.google.com/mail/u/0/#inbox/" + externalID
	}
	if strings.HasPrefix(addr, "@") {
		return "https://t.me/" + addr[1:]
	}
	return ""
}

// GetOrCreateUser — тот самый единственный пользователь установки. На пустой
// базе он создаётся сразу с демо-критериями: иначе первая же демо-лента
// оценивалась бы «на общих основаниях».
func GetOrCreateUser(db *postgres.DB) (*postgres.User, error) {
	user, err := db.LocalUser()
	if err != nil {
		return nil, err
	}
	if user.Criteria == "" {
		user.Criteria = DemoCriteria
		if err := db.SaveUser(user); err != nil {
			return nil, err
		}
	}
	return user, nil
}

// GetOrCreateConnection — источник демо-ленты вместе с состоянием из референса.
func GetOrCreateConnection(db *postgres.DB, user *postgres.User, kind string) (*postgres.Connection, error) {
	connection, err := db.GetOrCreateConnection(user.ID, kind)
	if err != nil {
		return nil, err
	}
	credentials, _ := json.Marshal(map[string]bool{"demo": true})
	now := postgres.UTCNow()
	connection.Account = Accounts[kind]
	connection.State = States[kind]
	connection.Credentials = string(credentials)
	connection.LastSyncAt = &now
	if err := db.SaveConnection(connection); err != nil {
		return nil, err
	}
	return connection, nil
}

// Seed заливает демо-ленту. Повторный запуск ничего не дублирует.
func Seed(db *postgres.DB, now time.Time) (int, error) {
	if now.IsZero() {
		now = postgres.UTCNow()
	}
	user, err := GetOrCreateUser(db)
	if err != nil {
		return 0, err
	}
	connections := map[string]*postgres.Connection{}
	for kind := range Accounts {
		connection, err := GetOrCreateConnection(db, user, kind)
		if err != nil {
			return 0, err
		}
		connections[kind] = connection
	}

	created := 0
	for _, item := range Messages {
		kind := srcToKind[item.Src]
		connection := connections[kind]
		externalID := "seed-" + strconv.Itoa(item.ID)
		if _, err := db.MessageByExternalID(connection.ID, externalID); err == nil {
			continue
		} else if !errors.Is(err, exceptions.ErrNotFound) {
			return created, err
		}
		analyzedAt := postgres.UTCNow()
		message := &postgres.Message{
			ConnectionID: connection.ID,
			ExternalID:   externalID,
			SenderName:   item.From,
			SenderAddr:   item.Addr,
			Subject:      item.Subj,
			Body:         item.Text,
			ReceivedAt:   ParseTime(item.Time, now),
			IsRead:       !item.Unread,
			Status:       "DONE",
			Level:        item.Level,
			Category:     item.Cat,
			DeadlineText: item.Deadline,
			NeedsReply:   item.Reply,
			NeedsAction:  item.Action,
			Summary:      item.Sum,
			ExternalURL:  externalURL(kind, item.Addr, externalID),
			AnalyzedAt:   &analyzedAt,
			Kind:         kind,
		}
		if err := db.InsertMessage(message); err != nil {
			return created, err
		}
		created++
	}
	return created, nil
}

// PlayLiveQueue проигрывает очередь «новых» сообщений: карточка появляется
// в PROCESSING и через пару секунд достраивается. Работает только внутри
// процесса сервера — события SSE живут в его памяти.
func PlayLiveQueue(db *postgres.DB, bus *events.Bus,
	firstDelay, interval, analyzeDelay time.Duration) {
	time.Sleep(firstDelay)
	for index, template := range LiveQueue {
		if err := playOne(db, bus, index, template, analyzeDelay); err != nil {
			return
		}
		if index < len(LiveQueue)-1 {
			time.Sleep(interval)
		}
	}
}

func playOne(db *postgres.DB, bus *events.Bus, index int,
	template LiveItem, analyzeDelay time.Duration) error {
	user, err := GetOrCreateUser(db)
	if err != nil {
		return err
	}
	kind := srcToKind[template.Src]
	connection, err := GetOrCreateConnection(db, user, kind)
	if err != nil {
		return err
	}
	externalID := "seed-live-" + strconv.Itoa(index)
	if _, err := db.MessageByExternalID(connection.ID, externalID); err == nil {
		return nil
	} else if !errors.Is(err, exceptions.ErrNotFound) {
		return err
	}

	message := &postgres.Message{
		ConnectionID: connection.ID,
		ExternalID:   externalID,
		SenderName:   template.From,
		SenderAddr:   template.Addr,
		Subject:      template.Subj,
		Body:         template.Text,
		ReceivedAt:   postgres.UTCNow(),
		IsRead:       false,
		Status:       "PROCESSING",
		ExternalURL:  externalURL(kind, template.Addr, externalID),
		Kind:         kind,
	}
	if err := db.InsertMessage(message); err != nil {
		return err
	}
	bus.Publish(user.ID, "message.created", schemas.MessageOut(message))

	time.Sleep(analyzeDelay)
	analyzedAt := postgres.UTCNow()
	message.Status = "DONE"
	message.Level = template.Res.Level
	message.Category = template.Res.Cat
	message.DeadlineText = template.Res.Deadline
	message.NeedsReply = template.Res.Reply
	message.NeedsAction = template.Res.Action
	message.Summary = template.Res.Sum
	message.AnalyzedAt = &analyzedAt
	if err := db.SaveMessage(message); err != nil {
		return err
	}
	bus.Publish(user.ID, "message.analyzed", schemas.MessageOut(message))
	return nil
}
