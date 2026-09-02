"""Шина событий для SSE. Один процесс, один пользователь — хватает кольцевого
буфера в памяти и опроса из генератора потока (docs/03-data-model.md, п. 6.1)."""
import threading
from collections import deque
from dataclasses import dataclass
from typing import Any


@dataclass(frozen=True)
class Event:
    seq: int
    user_id: int
    name: str  # message.created | message.analyzed
    data: dict[str, Any]


class EventBus:
    def __init__(self, maxlen: int = 200) -> None:
        self._events: deque[Event] = deque(maxlen=maxlen)
        self._lock = threading.Lock()
        self._seq = 0

    def publish(self, user_id: int, name: str, data: dict[str, Any]) -> Event:
        with self._lock:
            self._seq += 1
            event = Event(seq=self._seq, user_id=user_id, name=name, data=data)
            self._events.append(event)
        return event

    @property
    def cursor(self) -> int:
        """Текущая позиция: подписчик начинает с неё и не получает старое."""
        with self._lock:
            return self._seq

    def since(self, user_id: int, cursor: int) -> tuple[list[Event], int]:
        with self._lock:
            events = [e for e in self._events if e.seq > cursor and e.user_id == user_id]
            return events, self._seq

    def clear(self) -> None:
        with self._lock:
            self._events.clear()
            self._seq = 0


bus = EventBus()
