import { render, screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { App } from '../App';
import { api, ApiError } from '../lib/api';
import type { User } from '../lib/types';

/** Входа в приложение нет: ни формы, ни экрана, ни состояния «не вошёл»
 * (решение №50). Тесты сторожат то, что заняло его место. */

vi.mock('../lib/api', async () => {
  const actual = await vi.importActual<typeof import('../lib/api')>('../lib/api');
  return {
    ...actual,
    api: {
      me: vi.fn(),
      updateMe: vi.fn(),
      sources: vi.fn(),
      connections: vi.fn(),
    },
  };
});

const mocked = vi.mocked(api);

function makeUser(overrides: Partial<User> = {}): User {
  return {
    id: 1,
    criteria: 'Важны договоры и сроки.',
    theme: 'dark',
    density: 'spacious',
    created_at: '2026-09-03T10:00:00',
    ...overrides,
  };
}

beforeEach(() => {
  localStorage.clear();
  mocked.sources.mockResolvedValue([]);
  mocked.connections.mockResolvedValue([]);
});

afterEach(() => {
  vi.clearAllMocks();
});

describe('запуск без входа', () => {
  it('открывает ленту сразу, не спрашивая ничего', async () => {
    mocked.me.mockResolvedValue(makeUser());
    render(<App />);

    expect(await screen.findByRole('heading', { name: 'Лента' })).toBeInTheDocument();
    // Ни полей, ни кнопки входа на экране быть не может.
    expect(screen.queryByLabelText(/пароль/i)).not.toBeInTheDocument();
    expect(screen.queryByRole('button', { name: 'Войти' })).not.toBeInTheDocument();
  });

  it('называет причину, когда сервер не ответил', async () => {
    mocked.me.mockRejectedValue(new ApiError(0, 'Нет связи с сервером'));
    render(<App />);

    expect(await screen.findByText('Нет связи с сервером')).toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Повторить' })).toBeInTheDocument();
  });

  it('«Повторить» перезапрашивает профиль', async () => {
    mocked.me.mockRejectedValueOnce(new ApiError(0, 'Нет связи с сервером'));
    mocked.me.mockResolvedValueOnce(makeUser());
    render(<App />);

    await userEvent.click(await screen.findByRole('button', { name: 'Повторить' }));
    expect(await screen.findByRole('heading', { name: 'Лента' })).toBeInTheDocument();
  });
});

describe('онбординг', () => {
  it('спрашивает критерии на пустой установке', async () => {
    mocked.me.mockResolvedValue(makeUser({ criteria: '' }));
    render(<App />);

    expect(await screen.findByText('Что для вас важно?')).toBeInTheDocument();
  });

  it('после «Пропустить» больше не возвращается', async () => {
    mocked.me.mockResolvedValue(makeUser({ criteria: '' }));
    const first = render(<App />);

    await userEvent.click(await screen.findByRole('button', { name: 'Пропустить' }));
    expect(await screen.findByRole('heading', { name: 'Лента' })).toBeInTheDocument();
    // Критерии так и остались пустыми — отличить «пропустил» от «ещё не
    // спрашивали» можно только по отметке в localStorage.
    expect(mocked.updateMe).not.toHaveBeenCalled();

    first.unmount();
    render(<App />);
    expect(await screen.findByRole('heading', { name: 'Лента' })).toBeInTheDocument();
    expect(screen.queryByText('Что для вас важно?')).not.toBeInTheDocument();
  });

  it('не спрашивает, когда критерии уже заданы', async () => {
    mocked.me.mockResolvedValue(makeUser());
    render(<App />);

    expect(await screen.findByRole('heading', { name: 'Лента' })).toBeInTheDocument();
    expect(screen.queryByText('Что для вас важно?')).not.toBeInTheDocument();
  });

  it('«Начать анализ» сохраняет введённые критерии', async () => {
    mocked.me.mockResolvedValue(makeUser({ criteria: '' }));
    mocked.updateMe.mockResolvedValue({
      user: makeUser({ criteria: 'Важны письма от клиентов.' }),
      reanalyze_queued: 3,
    });
    render(<App />);

    await userEvent.type(
      await screen.findByLabelText('Критерии важности'),
      'Важны письма от клиентов.',
    );
    await userEvent.click(screen.getByRole('button', { name: 'Начать анализ' }));

    await waitFor(() =>
      expect(mocked.updateMe).toHaveBeenCalledWith({ criteria: 'Важны письма от клиентов.' }),
    );
    expect(await screen.findByRole('heading', { name: 'Лента' })).toBeInTheDocument();
  });
});
