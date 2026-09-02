/** Форматирование того, что бэкенд отдаёт «сырым»: время, инициалы, аватар. */

const WEEKDAYS = ['Вс', 'Пн', 'Вт', 'Ср', 'Чт', 'Пт', 'Сб'];
const MONTHS = [
  'янв.', 'февр.', 'марта', 'апр.', 'мая', 'июня',
  'июля', 'авг.', 'сент.', 'окт.', 'нояб.', 'дек.',
];

/** Бэкенд отдаёт время в UTC без суффикса — приводим к моменту явно. */
export function parseDate(iso: string): Date {
  return new Date(/[Z+]|-\d\d:\d\d$/.test(iso) ? iso : `${iso}Z`);
}

function sameDay(a: Date, b: Date): boolean {
  return (
    a.getFullYear() === b.getFullYear() &&
    a.getMonth() === b.getMonth() &&
    a.getDate() === b.getDate()
  );
}

function time(date: Date): string {
  return `${String(date.getHours()).padStart(2, '0')}:${String(date.getMinutes()).padStart(2, '0')}`;
}

/** «09:41» · «Вчера, 19:04» · «Пн» · «3 авг.» — как в референсе. */
export function formatTime(iso: string, now: Date = new Date()): string {
  const date = parseDate(iso);
  if (Number.isNaN(date.getTime())) return '';
  const yesterday = new Date(now);
  yesterday.setDate(now.getDate() - 1);

  if (sameDay(date, now)) return time(date);
  if (sameDay(date, yesterday)) return `Вчера, ${time(date)}`;

  const days = Math.floor((now.getTime() - date.getTime()) / 86_400_000);
  if (days >= 0 && days < 7) return WEEKDAYS[date.getDay()];
  return `${date.getDate()} ${MONTHS[date.getMonth()]}`;
}

/** «Синхронизировано 2 минуты назад» — подпись на экране «Источники». */
export function formatSyncedAgo(iso: string | null, now: Date = new Date()): string {
  if (!iso) return 'Ещё не синхронизировано';
  const minutes = Math.floor((now.getTime() - parseDate(iso).getTime()) / 60_000);
  if (minutes < 1) return 'Синхронизировано только что';
  if (minutes < 60) return `Синхронизировано ${minutes} ${plural(minutes, 'минуту', 'минуты', 'минут')} назад`;
  const hours = Math.floor(minutes / 60);
  if (hours < 24) return `Синхронизировано ${hours} ${plural(hours, 'час', 'часа', 'часов')} назад`;
  const days = Math.floor(hours / 24);
  return `Синхронизировано ${days} ${plural(days, 'день', 'дня', 'дней')} назад`;
}

export function plural(count: number, one: string, few: string, many: string): string {
  const mod100 = Math.abs(count) % 100;
  const mod10 = mod100 % 10;
  if (mod100 >= 11 && mod100 <= 14) return many;
  if (mod10 === 1) return one;
  if (mod10 >= 2 && mod10 <= 4) return few;
  return many;
}

export function messagesCount(total: number, unread: number): string {
  return `${total} ${plural(total, 'сообщение', 'сообщения', 'сообщений')} · ${unread} ${plural(
    unread,
    'непрочитанное',
    'непрочитанных',
    'непрочитанных',
  )}`;
}

/** Инициалы отправителя: «Дима · Продукт» → «ДП», «@samokat_bot» → «S». */
export function initials(name: string): string {
  const parts = name.replace(/[·@]/g, ' ').trim().split(/\s+/).filter(Boolean);
  if (parts.length === 0) return '?';
  const first = parts[0][0] ?? '';
  const second = parts[1]?.[0] ?? '';
  return (first + second).toUpperCase();
}

/** Шесть градиентов из референса. */
export const AVATAR_GRADIENTS = [
  'linear-gradient(150deg,#ff9f6a,#e0533d)',
  'linear-gradient(150deg,#67d6a8,#12876b)',
  'linear-gradient(150deg,#8fb5ff,#3a5fd0)',
  'linear-gradient(150deg,#e79bff,#8a2bc4)',
  'linear-gradient(150deg,#ffd06a,#e08a12)',
  'linear-gradient(150deg,#7fd8f0,#1a7fa8)',
];

/** Заливка выбирается из имени, чтобы один человек всегда был одного цвета. */
export function avatarGradient(name: string): string {
  let hash = 0;
  for (const character of name) {
    hash = (hash * 31 + character.charCodeAt(0)) % 997;
  }
  return AVATAR_GRADIENTS[hash % AVATAR_GRADIENTS.length];
}

/** Категория «—» приходит как заглушка при обработке и считается пустой. */
export function visibleCategory(category: string): string {
  const value = category.trim();
  return value && value !== '—' ? value : '';
}

/** Смещение часового пояса в минутах — для фильтра «Сегодня» на бэкенде. */
export function tzOffsetMinutes(now: Date = new Date()): number {
  return -now.getTimezoneOffset();
}
