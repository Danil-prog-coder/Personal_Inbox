import { useEffect, useRef, useState } from 'react';
import type { Message } from '../lib/types';

export interface StreamEvent {
  name: 'message.created' | 'message.analyzed';
  message: Message;
  /** Растёт с каждым событием: по нему подписчики понимают, что оно новое. */
  seq: number;
}

const POLL_INTERVAL = 30_000;

/**
 * Новые сообщения приезжают через SSE. Если поток недоступен, откатываемся
 * на опрос раз в 30 секунд (решение №18).
 */
export function useStream(enabled: boolean, onPoll: () => void): StreamEvent | null {
  const [event, setEvent] = useState<StreamEvent | null>(null);
  const pollRef = useRef(onPoll);
  const seqRef = useRef(0);
  pollRef.current = onPoll;

  useEffect(() => {
    if (!enabled) return;
    if (typeof EventSource === 'undefined') {
      const timer = window.setInterval(() => pollRef.current(), POLL_INTERVAL);
      return () => window.clearInterval(timer);
    }

    const source = new EventSource('/api/stream', { withCredentials: true });
    let fallback = 0;

    const handle = (name: StreamEvent['name']) => (raw: Event) => {
      try {
        const message = JSON.parse((raw as MessageEvent).data) as Message;
        seqRef.current += 1;
        setEvent({ name, message, seq: seqRef.current });
      } catch {
        // Битый кадр пропускаем: следующий придёт целым.
      }
    };

    const onCreated = handle('message.created');
    const onAnalyzed = handle('message.analyzed');
    source.addEventListener('message.created', onCreated);
    source.addEventListener('message.analyzed', onAnalyzed);

    source.onerror = () => {
      // EventSource переподключается сам; опрос страхует случай,
      // когда поток не поднимается вовсе.
      if (!fallback) {
        fallback = window.setInterval(() => pollRef.current(), POLL_INTERVAL);
      }
    };
    source.onopen = () => {
      if (fallback) {
        window.clearInterval(fallback);
        fallback = 0;
      }
    };

    return () => {
      source.removeEventListener('message.created', onCreated);
      source.removeEventListener('message.analyzed', onAnalyzed);
      source.close();
      if (fallback) window.clearInterval(fallback);
    };
  }, [enabled]);

  return event;
}
