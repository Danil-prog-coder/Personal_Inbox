"""Шина событий и SSE-поток."""
import asyncio

from backend.api.stream import _event_stream
from backend.events import EventBus, bus


def test_bus_gives_only_new_events_of_this_user():
    local = EventBus()
    cursor = local.cursor
    local.publish(1, "message.created", {"id": 1})
    local.publish(2, "message.created", {"id": 2})

    events, cursor = local.since(1, cursor)
    assert [event.data["id"] for event in events] == [1]
    # Повторный запрос с новым курсором ничего не возвращает.
    assert local.since(1, cursor)[0] == []


def test_bus_keeps_ring_buffer_bounded():
    local = EventBus(maxlen=3)
    for index in range(10):
        local.publish(1, "message.created", {"id": index})
    events, _ = local.since(1, 0)
    assert [event.data["id"] for event in events] == [7, 8, 9]


def test_event_stream_sends_new_events(user):
    """Генератор потока отдаёт событие, появившееся после подписки,
    и не отдаёт события чужого пользователя."""

    async def run() -> tuple[str, str]:
        stream = _event_stream(user.id)
        hello = await stream.__anext__()
        bus.publish(user.id + 1, "message.created", {"id": 1})
        bus.publish(user.id, "message.created", {"id": 42, "subject": "Новое письмо"})
        chunk = await asyncio.wait_for(stream.__anext__(), timeout=5)
        await stream.aclose()
        return hello, chunk

    hello, chunk = asyncio.run(run())
    assert hello.startswith(":")  # первый байт, чтобы браузер не держал соединение вслепую
    assert chunk.startswith("event: message.created")
    assert '"subject": "Новое письмо"' in chunk
    assert '"id": 1' not in chunk


def test_stream_requires_auth(client):
    assert client.get("/api/stream").status_code == 401
