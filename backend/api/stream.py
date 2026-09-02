"""Server-Sent Events: новые сообщения и результаты оценки приезжают на фронт.
WebSocket не нужен — поток односторонний (решение №18)."""
import asyncio
import json

from fastapi import APIRouter, Depends
from fastapi.responses import StreamingResponse

from backend.events import bus
from backend.models import User
from backend.security import current_user

router = APIRouter(prefix="/api", tags=["stream"])

POLL_SECONDS = 1.0
PING_SECONDS = 15.0


async def _event_stream(user_id: int):
    cursor = bus.cursor  # старые события новому подписчику не отдаём
    since_ping = 0.0
    # Первый байт сразу — иначе браузер держит соединение «висящим».
    yield ": ok\n\n"
    while True:
        events, cursor = bus.since(user_id, cursor)
        for event in events:
            payload = json.dumps(event.data, ensure_ascii=False)
            yield f"event: {event.name}\ndata: {payload}\n\n"
        if events:
            since_ping = 0.0
        await asyncio.sleep(POLL_SECONDS)
        since_ping += POLL_SECONDS
        if since_ping >= PING_SECONDS:
            since_ping = 0.0
            yield ": ping\n\n"


@router.get("/stream")
async def stream(user: User = Depends(current_user)) -> StreamingResponse:
    return StreamingResponse(
        _event_stream(user.id),
        media_type="text/event-stream",
        headers={"Cache-Control": "no-cache", "X-Accel-Buffering": "no"},
    )
