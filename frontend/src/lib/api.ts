/** Обёртки над HTTP API. Входа нет: приложение локальное и обслуживает одного
 * пользователя, сервер узнаёт его сам (решение №50). credentials оставлены —
 * запрос уходит на тот же origin и лишними не бывают. */
import type {
  Connection,
  Density,
  Filters,
  Level,
  Message,
  MessageList,
  SourceCard,
  SourceKind,
  Summary,
  SummaryPeriod,
  Theme,
  User,
} from './types';
import { tzOffsetMinutes } from './format';

export class ApiError extends Error {
  status: number;

  constructor(status: number, message: string) {
    super(message);
    this.status = status;
  }
}

async function request<T>(path: string, options: RequestInit = {}): Promise<T> {
  let response: Response;
  try {
    response = await fetch(path, {
      credentials: 'include',
      headers: options.body ? { 'Content-Type': 'application/json' } : undefined,
      ...options,
    });
  } catch {
    throw new ApiError(0, 'Нет связи с сервером');
  }
  if (response.status === 204) return undefined as T;
  const text = await response.text();
  const data = text ? JSON.parse(text) : null;
  if (!response.ok) {
    throw new ApiError(response.status, errorMessage(data, response.status));
  }
  return data as T;
}

function errorMessage(data: unknown, status: number): string {
  if (data && typeof data === 'object' && 'detail' in data) {
    const detail = (data as { detail: unknown }).detail;
    if (typeof detail === 'string') return detail;
    // Ошибка валидации Pydantic приходит списком — показываем первую причину.
    if (Array.isArray(detail) && detail.length > 0) {
      const first = detail[0] as { msg?: string };
      if (first?.msg) return first.msg;
    }
  }
  return status === 0 ? 'Нет связи с сервером' : 'Что-то пошло не так';
}

const json = (body: unknown): RequestInit => ({ method: 'POST', body: JSON.stringify(body) });

export const api = {
  me: () => request<User>('/api/me'),

  updateMe: (patch: { criteria?: string; theme?: Theme; density?: Density }) =>
    request<{ user: User; reanalyze_queued: number }>('/api/me', {
      method: 'PATCH',
      body: JSON.stringify(patch),
    }),

  connections: () => request<Connection[]>('/api/connections'),

  gmailAuthUrl: () => request<{ auth_url: string }>('/api/connections/gmail/start', { method: 'POST' }),

  // Подключение Telegram идёт в два шага: номер → код из Telegram
  // (и пароль, если включена двухфакторная защита).
  telegramStart: (phone: string) =>
    request<{ phone: string }>('/api/connections/telegram/start', json({ phone })),

  // Ответ либо подключение, либо просьба ввести пароль двухфакторной защиты.
  telegramConfirm: (code: string, password: string) =>
    request<Connection | { password_needed: true }>(
      '/api/connections/telegram/confirm',
      json({ code, password }),
    ),

  disconnect: (kind: SourceKind) =>
    request<void>(`/api/connections/${kind}`, { method: 'DELETE' }),

  sources: () => request<SourceCard[]>('/api/sources'),

  messages: (source: SourceKind, filters: Filters, query: string) => {
    const params = new URLSearchParams({
      source,
      level: filters.level,
      status: filters.status,
      reply: filters.reply,
      action: filters.action,
      period: filters.period,
      tz_offset: String(tzOffsetMinutes()),
    });
    if (query.trim()) params.set('q', query.trim());
    return request<MessageList>(`/api/messages?${params.toString()}`);
  },

  message: (id: number) => request<Message>(`/api/messages/${id}`),

  markRead: (id: number) => request<Message>(`/api/messages/${id}/read`, { method: 'POST' }),

  setLevel: (id: number, level: Level) =>
    request<Message>(`/api/messages/${id}/level`, json({ level })),

  summary: (period: SummaryPeriod) =>
    request<Summary>(`/api/summary?period=${period}`),
};
