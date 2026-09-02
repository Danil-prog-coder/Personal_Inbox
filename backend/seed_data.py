"""Демо-данные из референса: 19 сообщений ленты и 3 сообщения очереди живой
демонстрации. Перенесены дословно (docs/03-data-model.md, п. 7) — набор специально
разнородный, на нём проверяются крайние случаи вёрстки. Не сокращать и не заменять.

Запуск:  python -m backend.seed_data          — залить пользователя, источники и ленту
         python -m backend.seed_data --live   — доиграть очередь «новых» сообщений
"""
import argparse
import json
import time
from datetime import datetime, timedelta

from sqlalchemy import select
from sqlalchemy.orm import Session

from backend.db import Base, SessionLocal, engine
from backend.events import bus
from backend.models import Connection, Message, User, utcnow
from backend.security import hash_password
from backend.serializers import message_out

DEMO_EMAIL = "demo@personal.inbox"
DEMO_PASSWORD = "demo12345"
DEMO_CRITERIA = (
    "Важны письма от клиентов Northline, всё про договоры и сроки, "
    "сообщения от команды с блокерами. Рассылки и уведомления сервисов — неважно."
)
ACCOUNTS = {"gmail": "me@northline.io", "telegram": "@maxorlov"}
# В референсе Telegram показан как требующий повторной авторизации — состояние
# reauth нужно, чтобы на демо-данных было видно предупреждение на карточке.
STATES = {"gmail": "active", "telegram": "reauth"}
SRC_TO_KIND = {"gmail": "gmail", "tg": "telegram"}

MESSAGES = [{'id': 1,
  'src': 'gmail',
  'from': 'Анна Ковалёва',
  'addr': 'a.kovaleva@northline.io',
  'time': '09:41',
  'subj': 'Договор Northline — правки до конца дня',
  'text': 'Юристы вернули версию с комментариями. Нужна ваша реакция по пунктам 4.2 и 7 до 18:00, '
          'иначе подписание уезжает на следующую неделю.',
  'level': 'CRITICAL',
  'action': True,
  'reply': True,
  'unread': True,
  'cat': 'Юридическое',
  'deadline': 'Сегодня, 18:00',
  'sum': 'Правки к договору требуют вашего решения по двум пунктам до 18:00 сегодня.',
  'status': 'done'},
 {'id': 2,
  'src': 'gmail',
  'from': 'Налоговая служба',
  'addr': 'noreply@nalog.gov',
  'time': '08:52',
  'subj': 'Уведомление: срок подачи декларации',
  'text': 'Напоминаем о необходимости подать декларацию за отчётный период до 30 сентября. При '
          'нарушении срока начисляется штраф.',
  'level': 'HIGH',
  'action': True,
  'reply': False,
  'unread': True,
  'cat': 'Документы',
  'deadline': '30 сентября',
  'sum': 'Декларацию нужно подать до 30 сентября, иначе штраф.',
  'status': 'done'},
 {'id': 3,
  'src': 'gmail',
  'from': 'Stripe',
  'addr': 'no-reply@stripe.com',
  'time': '08:30',
  'subj': 'Платёж на $1 240.00 прошёл успешно',
  'text': 'Ваш ежемесячный платёж по подписке Northline Pro обработан. Квитанция во вложении, '
          'следующее списание 2 октября.',
  'level': 'NORMAL',
  'action': False,
  'reply': False,
  'unread': False,
  'cat': 'Финансы',
  'deadline': '',
  'sum': 'Ежемесячный платёж $1 240.00 успешно прошёл.',
  'status': 'done'},
 {'id': 4,
  'src': 'gmail',
  'from': 'Ольга Тимченко',
  'addr': 'o.timchenko@hr-lab.ru',
  'time': 'Вчера, 19:04',
  'subj': 'Оффер senior-разработчику — нужна ваша виза',
  'text': 'Финальный кандидат ждёт подтверждения вилки: 380–420 тыс. Он держит второй оффер до '
          'вторника, поэтому решение хорошо бы принять завтра.',
  'level': 'HIGH',
  'action': True,
  'reply': True,
  'unread': True,
  'cat': 'Найм',
  'deadline': 'Вторник',
  'sum': 'Требуется согласование вилки по офферу; кандидат держит второй оффер до вторника.',
  'status': 'done'},
 {'id': 5,
  'src': 'gmail',
  'from': 'AWS Billing',
  'addr': 'aws-billing@amazon.com',
  'time': 'Вчера, 16:20',
  'subj': 'Прогноз счёта превышен на 42%',
  'text': 'Текущее потребление по аккаунту 4471-9902 превышает бюджет месяца. Основной рост — RDS '
          'и исходящий трафик CloudFront.',
  'level': 'HIGH',
  'action': True,
  'reply': False,
  'unread': False,
  'cat': 'Инфраструктура',
  'deadline': '',
  'sum': 'Расход по AWS выше бюджета на 42%, рост в RDS и CloudFront.',
  'status': 'done'},
 {'id': 6,
  'src': 'gmail',
  'from': 'Мария Соколова',
  'addr': 'm.sokolova@northline.io',
  'time': 'Вчера, 14:10',
  'subj': 'Макеты онбординга — три варианта',
  'text': 'Закинула в папку три направления: минимальное, с прогрессом и с примерами. Посмотри, '
          'когда будет минутка — не срочно, до ревью в пятницу.',
  'level': 'NORMAL',
  'action': False,
  'reply': True,
  'unread': False,
  'cat': 'Дизайн',
  'deadline': 'Пятница',
  'sum': 'Три варианта макетов онбординга готовы, ревью в пятницу.',
  'status': 'done'},
 {'id': 7,
  'src': 'gmail',
  'from': 'Сбербанк Бизнес',
  'addr': 'info@sberbank.ru',
  'time': 'Вчера, 11:37',
  'subj': 'Выписка по счёту за август',
  'text': 'Выписка сформирована и доступна в личном кабинете. Обороты за месяц — 4,8 млн ₽.',
  'level': 'NORMAL',
  'action': False,
  'reply': False,
  'unread': False,
  'cat': 'Финансы',
  'deadline': '',
  'sum': 'Августовская выписка готова, обороты 4,8 млн ₽.',
  'status': 'done'},
 {'id': 8,
  'src': 'gmail',
  'from': 'Product Weekly',
  'addr': 'digest@productweekly.co',
  'time': 'Пн',
  'subj': 'Выпуск 214: что нового в AI-инструментах',
  'text': 'Еженедельная подборка: девять разборов, интервью с командой Linear и колонка про '
          'приоритизацию бэклога.',
  'level': 'LOW',
  'action': False,
  'reply': False,
  'unread': False,
  'cat': 'Рассылка',
  'deadline': '',
  'sum': 'Регулярный дайджест, действий не требует.',
  'status': 'done'},
 {'id': 9,
  'src': 'gmail',
  'from': 'Figma',
  'addr': 'updates@figma.com',
  'time': 'Пн',
  'subj': 'Ваш план Organization продлён на год',
  'text': 'Спасибо, что остаётесь с нами. Счёт и детали тарифа доступны в настройках биллинга.',
  'level': 'LOW',
  'action': False,
  'reply': False,
  'unread': False,
  'cat': 'Сервисы',
  'deadline': '',
  'sum': 'Подписка Figma продлена автоматически.',
  'status': 'done'},
 {'id': 10,
  'src': 'gmail',
  'from': 'Кирилл Ушаков',
  'addr': 'k.ushakov@vendorlab.io',
  'time': 'Вс',
  'subj': 'Коммерческое предложение по интеграции',
  'text': 'Подготовили расчёт под ваш объём: 2,1 млн ₽ за внедрение и 180 тыс. ₽ в месяц. Готовы '
          'обсудить на звонке.',
  'level': 'NORMAL',
  'action': False,
  'reply': True,
  'unread': False,
  'cat': 'Партнёры',
  'deadline': '',
  'sum': 'Вендор прислал расчёт: 2,1 млн ₽ внедрение, 180 тыс. ₽ в месяц.',
  'status': 'done'},
 {'id': 11,
  'src': 'gmail',
  'from': 'Google Workspace',
  'addr': 'security@google.com',
  'time': 'Сб',
  'subj': 'Новый вход с устройства MacBook Pro',
  'text': 'Обнаружен вход в аккаунт из Москвы. Если это были вы, действий не требуется.',
  'level': 'LOW',
  'action': False,
  'reply': False,
  'unread': False,
  'cat': 'Безопасность',
  'deadline': '',
  'sum': 'Уведомление о входе с нового устройства.',
  'status': 'done'},
 {'id': 12,
  'src': 'tg',
  'from': 'Инвесторы · Seed round',
  'addr': 'групповой чат, 9 участников',
  'time': '09:22',
  'subj': 'Питч-дек — шесть вопросов от фонда',
  'text': 'Прислали вопросы по юнит-экономике: CAC по каналам, retention 6+ месяцев и структура '
          'косвенных расходов. Ждут ответ к среде.',
  'level': 'CRITICAL',
  'action': True,
  'reply': True,
  'unread': True,
  'cat': 'Инвестиции',
  'deadline': 'Среда',
  'sum': 'Фонд ждёт ответы по юнит-экономике к среде.',
  'status': 'done'},
 {'id': 13,
  'src': 'tg',
  'from': 'Дима · Продукт',
  'addr': '@dmitry_pm',
  'time': '09:12',
  'subj': 'Релиз 2.4 — блокер на бэкенде',
  'text': 'Ребята нашли гонку в очереди задач: при пиковой нагрузке часть событий обрабатывается '
          'дважды. Предлагаю сдвинуть релиз на завтра утро. Ок?',
  'level': 'HIGH',
  'action': True,
  'reply': True,
  'unread': True,
  'cat': 'Разработка',
  'deadline': 'Сегодня',
  'sum': 'Просят подтвердить перенос релиза 2.4 на завтра из-за блокера.',
  'status': 'done'},
 {'id': 14,
  'src': 'tg',
  'from': 'Northline · дежурство',
  'addr': '@northline_alerts',
  'time': '07:48',
  'subj': 'API p95 вырос до 1,8 с',
  'text': 'Алерт по латентности держится 20 минут. Дежурный смотрит, похоже на медленный запрос в '
          'отчётах.',
  'level': 'HIGH',
  'action': False,
  'reply': False,
  'unread': True,
  'cat': 'Инфраструктура',
  'deadline': '',
  'sum': 'Латентность API выросла, дежурный уже разбирается.',
  'status': 'done'},
 {'id': 15,
  'src': 'tg',
  'from': 'Саша Ким',
  'addr': '@sasha_k',
  'time': 'Вчера, 21:30',
  'subj': 'Кофе на неделе?',
  'text': 'Я в городе до пятницы, давай пересечёмся — расскажу, чем закончилась история с их '
          'раундом.',
  'level': 'NORMAL',
  'action': False,
  'reply': True,
  'unread': False,
  'cat': 'Личное',
  'deadline': 'Пятница',
  'sum': 'Предлагает встретиться до пятницы.',
  'status': 'done'},
 {'id': 16,
  'src': 'tg',
  'from': 'Лена · Бухгалтерия',
  'addr': '@lena_acc',
  'time': 'Вчера, 17:02',
  'subj': 'Нужны закрывающие по трём подрядчикам',
  'text': 'Без актов не закрою квартал. Скинь, пожалуйста, до конца недели — иначе перенесётся в '
          'следующий период.',
  'level': 'HIGH',
  'action': True,
  'reply': True,
  'unread': False,
  'cat': 'Финансы',
  'deadline': 'Пятница',
  'sum': 'Нужны закрывающие документы по трём подрядчикам до конца недели.',
  'status': 'done'},
 {'id': 17,
  'src': 'tg',
  'from': 'Дизайн-ревью',
  'addr': 'групповой чат, 5 участников',
  'time': 'Пн',
  'subj': 'Обсудили новую навигацию',
  'text': 'Сошлись на варианте с боковой панелью. Максим, нужен твой финальный голос перед '
          'разработкой.',
  'level': 'NORMAL',
  'action': True,
  'reply': True,
  'unread': False,
  'cat': 'Дизайн',
  'deadline': '',
  'sum': 'Команда выбрала боковую навигацию, ждут ваш голос.',
  'status': 'done'},
 {'id': 18,
  'src': 'tg',
  'from': 'Спортзал «Форма»',
  'addr': '@formagym',
  'time': 'Сб',
  'subj': 'Абонемент заканчивается через 5 дней',
  'text': 'Продлите до 7 сентября, чтобы сохранить текущую цену — с октября тарифы вырастут.',
  'level': 'LOW',
  'action': False,
  'reply': False,
  'unread': False,
  'cat': 'Личное',
  'deadline': 'Через 5 дней',
  'sum': 'Абонемент истекает через 5 дней.',
  'status': 'done'},
 {'id': 19,
  'src': 'tg',
  'from': 'Доставка «Самокат»',
  'addr': '@samokat_bot',
  'time': 'Сб',
  'subj': 'Заказ доставлен',
  'text': 'Курьер оставил заказ у двери. Спасибо, что выбираете нас!',
  'level': 'LOW',
  'action': False,
  'reply': False,
  'unread': False,
  'cat': 'Сервисы',
  'deadline': '',
  'sum': 'Заказ доставлен, действий не требуется.',
  'status': 'done'}]

LIVE_QUEUE = [{'src': 'gmail',
  'from': 'Виктор Лебедев',
  'addr': 'v.lebedev@northline.io',
  'subj': 'Re: Договор Northline — согласовал п. 7',
  'text': 'Принял вашу формулировку по седьмому пункту, остаётся 4.2. Если пришлёте до вечера, '
          'подпишем сегодня.',
  'res': {'level': 'CRITICAL',
          'action': True,
          'reply': True,
          'cat': 'Юридическое',
          'deadline': 'Сегодня, 18:00',
          'sum': 'Пункт 7 согласован, остаётся решение по 4.2 — подпись сегодня.'}},
 {'src': 'tg',
  'from': 'Дима · Продукт',
  'addr': '@dmitry_pm',
  'subj': 'Фикс блокера в мастере',
  'text': 'Гонку в очереди починили, тесты зелёные. Возвращаем релиз на сегодняшний вечер?',
  'res': {'level': 'HIGH',
          'action': True,
          'reply': True,
          'cat': 'Разработка',
          'deadline': 'Сегодня',
          'sum': 'Блокер починен, спрашивают про возврат релиза на вечер.'}},
 {'src': 'gmail',
  'from': 'Notion',
  'addr': 'team@notion.so',
  'subj': 'Вас упомянули в «Q4 планирование»',
  'text': 'Мария Соколова оставила комментарий с вашим упоминанием в разделе «Ресурсы дизайна».',
  'res': {'level': 'LOW',
          'action': False,
          'reply': False,
          'cat': 'Сервисы',
          'deadline': '',
          'sum': 'Упоминание в документе, действий не требует.'}}]


def _parse_time(spec: str, now: datetime) -> datetime:
    """Время из референса («09:41», «Вчера, 19:04», «Пн») — в реальную метку.
    В БД хранится datetime, строка вычисляется на фронте (docs/03-data-model.md)."""
    weekdays = {"Пн": 0, "Вт": 1, "Ср": 2, "Чт": 3, "Пт": 4, "Сб": 5, "Вс": 6}
    spec = spec.strip()
    if spec in weekdays:
        delta = (now.weekday() - weekdays[spec]) % 7 or 7
        day = now - timedelta(days=delta)
        return day.replace(hour=12, minute=0, second=0, microsecond=0)
    day = now
    if spec.startswith("Вчера"):
        day = now - timedelta(days=1)
        spec = spec.split(",", 1)[1].strip() if "," in spec else "12:00"
    try:
        hour, minute = (int(part) for part in spec.split(":", 1))
    except ValueError:
        return day.replace(hour=12, minute=0, second=0, microsecond=0)
    return day.replace(hour=hour, minute=minute, second=0, microsecond=0)


def _external_url(kind: str, addr: str, external_id: str) -> str:
    if kind == "gmail":
        return f"https://mail.google.com/mail/u/0/#inbox/{external_id}"
    # У группового чата без @username прямой ссылки нет — кнопка не показывается.
    return f"https://t.me/{addr[1:]}" if addr.startswith("@") else ""


def get_or_create_user(db: Session) -> User:
    user = db.execute(select(User).where(User.email == DEMO_EMAIL)).scalar_one_or_none()
    if user is None:
        user = User(
            email=DEMO_EMAIL,
            password_hash=hash_password(DEMO_PASSWORD),
            criteria=DEMO_CRITERIA,
        )
        db.add(user)
        db.commit()
        db.refresh(user)
    return user


def get_or_create_connection(db: Session, user: User, kind: str) -> Connection:
    connection = db.execute(
        select(Connection).where(Connection.user_id == user.id, Connection.kind == kind)
    ).scalar_one_or_none()
    if connection is None:
        connection = Connection(user_id=user.id, kind=kind)
        db.add(connection)
    connection.account = ACCOUNTS[kind]
    connection.state = STATES[kind]
    connection.credentials = json.dumps({"demo": True})
    connection.last_sync_at = utcnow()
    db.commit()
    db.refresh(connection)
    return connection


def seed(db: Session, now: datetime | None = None) -> int:
    """Залить демо-ленту. Повторный запуск ничего не дублирует."""
    now = now or utcnow()
    user = get_or_create_user(db)
    connections = {kind: get_or_create_connection(db, user, kind) for kind in ACCOUNTS}

    created = 0
    for item in MESSAGES:
        kind = SRC_TO_KIND[item["src"]]
        connection = connections[kind]
        external_id = f"seed-{item['id']}"
        exists = db.execute(
            select(Message).where(
                Message.connection_id == connection.id, Message.external_id == external_id
            )
        ).scalar_one_or_none()
        if exists is not None:
            continue
        db.add(
            Message(
                connection_id=connection.id,
                external_id=external_id,
                sender_name=item["from"],
                sender_addr=item["addr"],
                subject=item["subj"],
                body=item["text"],
                received_at=_parse_time(item["time"], now),
                is_read=not item["unread"],
                status="DONE",
                level=item["level"],
                category=item["cat"],
                deadline_text=item["deadline"],
                needs_reply=item["reply"],
                needs_action=item["action"],
                summary=item["sum"],
                external_url=_external_url(kind, item["addr"], external_id),
                analyzed_at=utcnow(),
            )
        )
        created += 1
    db.commit()
    return created


def play_live_queue(first_delay: float = 6.0, interval: float = 16.0, analyze_delay: float = 2.6):
    """Проиграть очередь «новых» сообщений: карточка появляется в PROCESSING,
    через ~2.6 с достраивается. Работает только внутри процесса сервера —
    события SSE живут в его памяти."""
    time.sleep(first_delay)
    for index, template in enumerate(LIVE_QUEUE):
        with SessionLocal() as db:
            user = get_or_create_user(db)
            kind = SRC_TO_KIND[template["src"]]
            connection = get_or_create_connection(db, user, kind)
            external_id = f"seed-live-{index}"
            exists = db.execute(
                select(Message).where(
                    Message.connection_id == connection.id, Message.external_id == external_id
                )
            ).scalar_one_or_none()
            if exists is not None:
                continue
            message = Message(
                connection_id=connection.id,
                external_id=external_id,
                sender_name=template["from"],
                sender_addr=template["addr"],
                subject=template["subj"],
                body=template["text"],
                received_at=utcnow(),
                is_read=False,
                status="PROCESSING",
                external_url=_external_url(kind, template["addr"], external_id),
            )
            db.add(message)
            db.commit()
            db.refresh(message)
            bus.publish(user.id, "message.created", message_out(message).model_dump(mode="json"))

            time.sleep(analyze_delay)
            result = template["res"]
            message.status = "DONE"
            message.level = result["level"]
            message.category = result["cat"]
            message.deadline_text = result["deadline"]
            message.needs_reply = result["reply"]
            message.needs_action = result["action"]
            message.summary = result["sum"]
            message.analyzed_at = utcnow()
            db.commit()
            db.refresh(message)
            bus.publish(user.id, "message.analyzed", message_out(message).model_dump(mode="json"))
        if index < len(LIVE_QUEUE) - 1:
            time.sleep(interval)


def main() -> None:
    parser = argparse.ArgumentParser(description="Залить демо-данные Personal Inbox")
    parser.add_argument("--live", action="store_true", help="доиграть очередь новых сообщений")
    args = parser.parse_args()

    Base.metadata.create_all(bind=engine)
    with SessionLocal() as db:
        created = seed(db)
    print(f"Демо-лента: добавлено сообщений — {created}")
    print(f"Вход: {DEMO_EMAIL} / {DEMO_PASSWORD}")
    if args.live:
        print("Очередь живой демонстрации: 3 сообщения")
        play_live_queue(first_delay=1.0, interval=3.0)


if __name__ == "__main__":
    main()
