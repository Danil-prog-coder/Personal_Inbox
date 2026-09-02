import { describe, expect, it } from 'vitest';
import {
  avatarGradient,
  formatSyncedAgo,
  formatTime,
  initials,
  messagesCount,
  plural,
  visibleCategory,
} from '../lib/format';

const NOW = new Date(2026, 8, 2, 15, 0); // среда, 2 сентября 2026

function local(year: number, month: number, day: number, hour: number, minute: number): string {
  // Бэкенд отдаёт UTC без суффикса, поэтому строим такую же строку.
  const date = new Date(Date.UTC(year, month, day, hour, minute));
  return date.toISOString().slice(0, 19);
}

describe('formatTime', () => {
  it('сегодняшнее сообщение показывает временем', () => {
    const iso = new Date(NOW.getTime() - 3 * 3600_000).toISOString().replace('Z', '');
    expect(formatTime(iso, NOW)).toMatch(/^\d{2}:\d{2}$/);
  });

  it('вчерашнее — с подписью «Вчера»', () => {
    const iso = new Date(NOW.getTime() - 20 * 3600_000).toISOString().replace('Z', '');
    expect(formatTime(iso, NOW)).toMatch(/^Вчера, \d{2}:\d{2}$/);
  });

  it('на этой неделе — днём недели', () => {
    const iso = new Date(NOW.getTime() - 3 * 86_400_000).toISOString().replace('Z', '');
    expect(formatTime(iso, NOW)).toMatch(/^(Пн|Вт|Ср|Чт|Пт|Сб|Вс)$/);
  });

  it('старое — датой с месяцем', () => {
    expect(formatTime(local(2026, 6, 14, 9, 0), NOW)).toBe('14 июля');
  });

  it('битую дату не рисует', () => {
    expect(formatTime('совсем не дата', NOW)).toBe('');
  });
});

describe('initials', () => {
  it('берёт первые буквы двух слов', () => {
    expect(initials('Анна Ковалёва')).toBe('АК');
  });

  it('разбирает имя с разделителем из референса', () => {
    expect(initials('Дима · Продукт')).toBe('ДП');
  });

  it('работает с одним словом и с ником', () => {
    expect(initials('Stripe')).toBe('S');
    expect(initials('@samokat_bot')).toBe('S');
  });

  it('не падает на пустой строке', () => {
    expect(initials('   ')).toBe('?');
  });
});

describe('avatarGradient', () => {
  it('один и тот же человек всегда одного цвета', () => {
    expect(avatarGradient('Анна Ковалёва')).toBe(avatarGradient('Анна Ковалёва'));
  });

  it('разные имена обычно расходятся по цветам', () => {
    const unique = new Set(
      ['Анна', 'Дима', 'Лена', 'Stripe', 'Figma', 'Ольга'].map(avatarGradient),
    );
    expect(unique.size).toBeGreaterThan(1);
  });
});

describe('plural', () => {
  it('склоняет по правилам русского языка', () => {
    expect(plural(1, 'сообщение', 'сообщения', 'сообщений')).toBe('сообщение');
    expect(plural(3, 'сообщение', 'сообщения', 'сообщений')).toBe('сообщения');
    expect(plural(11, 'сообщение', 'сообщения', 'сообщений')).toBe('сообщений');
    expect(plural(21, 'сообщение', 'сообщения', 'сообщений')).toBe('сообщение');
    expect(plural(0, 'сообщение', 'сообщения', 'сообщений')).toBe('сообщений');
  });
});

describe('messagesCount', () => {
  it('собирает подзаголовок ленты', () => {
    expect(messagesCount(19, 6)).toBe('19 сообщений · 6 непрочитанных');
    expect(messagesCount(1, 1)).toBe('1 сообщение · 1 непрочитанное');
  });
});

describe('visibleCategory', () => {
  it('заглушку «—» считает пустой категорией', () => {
    expect(visibleCategory('—')).toBe('');
    expect(visibleCategory('  ')).toBe('');
    expect(visibleCategory('Финансы')).toBe('Финансы');
  });
});

describe('formatSyncedAgo', () => {
  it('считает минуты, часы и дни', () => {
    const minutesAgo = new Date(NOW.getTime() - 2 * 60_000).toISOString().replace('Z', '');
    expect(formatSyncedAgo(minutesAgo, NOW)).toBe('Синхронизировано 2 минуты назад');
    const hoursAgo = new Date(NOW.getTime() - 3 * 3600_000).toISOString().replace('Z', '');
    expect(formatSyncedAgo(hoursAgo, NOW)).toBe('Синхронизировано 3 часа назад');
  });

  it('без синхронизации говорит об этом прямо', () => {
    expect(formatSyncedAgo(null, NOW)).toBe('Ещё не синхронизировано');
  });
});
