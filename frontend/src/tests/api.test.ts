import { afterEach, describe, expect, it, vi } from 'vitest';
import { api, ApiError } from '../lib/api';
import { EMPTY_FILTERS } from '../lib/types';

interface FakeResponse {
  ok?: boolean;
  status?: number;
  body?: unknown;
}

function mockFetch(response: FakeResponse) {
  const fetchMock = vi.fn(async () => ({
    ok: response.ok ?? true,
    status: response.status ?? 200,
    text: async () => (response.body === undefined ? '' : JSON.stringify(response.body)),
  })) as unknown as typeof fetch;
  vi.stubGlobal('fetch', fetchMock);
  return fetchMock as unknown as ReturnType<typeof vi.fn>;
}

afterEach(() => {
  vi.unstubAllGlobals();
});

describe('api.messages', () => {
  it('передаёт все фильтры и часовой пояс', async () => {
    const fetchMock = mockFetch({ body: { items: [], total: 0, unread: 0 } });
    await api.messages(
      'telegram',
      { ...EMPTY_FILTERS, level: 'HIGH', status: 'unread', period: 'week' },
      '  договор  ',
    );

    const url = new URL(fetchMock.mock.calls[0][0] as string, 'http://localhost');
    expect(url.pathname).toBe('/api/messages');
    expect(url.searchParams.get('source')).toBe('telegram');
    expect(url.searchParams.get('level')).toBe('HIGH');
    expect(url.searchParams.get('status')).toBe('unread');
    expect(url.searchParams.get('period')).toBe('week');
    expect(url.searchParams.get('q')).toBe('договор');
    expect(url.searchParams.has('tz_offset')).toBe(true);
  });

  it('пустой запрос не отправляет вовсе', async () => {
    const fetchMock = mockFetch({ body: { items: [], total: 0, unread: 0 } });
    await api.messages('gmail', EMPTY_FILTERS, '   ');
    const url = new URL(fetchMock.mock.calls[0][0] as string, 'http://localhost');
    expect(url.searchParams.has('q')).toBe(false);
  });
});

describe('обработка ошибок', () => {
  it('показывает текст detail от бэкенда', async () => {
    mockFetch({ ok: false, status: 401, body: { detail: 'Неверный email или пароль' } });
    await expect(api.login('me@northline.io', 'нет')).rejects.toMatchObject({
      status: 401,
      message: 'Неверный email или пароль',
    });
  });

  it('разбирает ошибку валидации со списком причин', async () => {
    mockFetch({
      ok: false,
      status: 422,
      body: { detail: [{ msg: 'String should have at least 8 characters' }] },
    });
    await expect(api.register('me@northline.io', 'мало')).rejects.toBeInstanceOf(ApiError);
  });

  it('обрыв сети превращает в понятную ошибку', async () => {
    vi.stubGlobal(
      'fetch',
      vi.fn(async () => {
        throw new TypeError('failed to fetch');
      }),
    );
    await expect(api.me()).rejects.toMatchObject({ status: 0, message: 'Нет связи с сервером' });
  });

  it('204 не пытается разобрать тело', async () => {
    mockFetch({ status: 204 });
    await expect(api.logout()).resolves.toBeUndefined();
  });
});
