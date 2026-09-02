import type { Filters } from '../lib/types';

/** Пять групп чипов, в каждой активен ровно один (docs/02-screens.md, п. 4.3). */
const GROUPS: { key: keyof Filters; label: string; options: [string, string][] }[] = [
  {
    key: 'level',
    label: 'Важность',
    options: [
      ['all', 'Любая'],
      ['CRITICAL', 'Критично'],
      ['HIGH', 'Важно'],
      ['NORMAL', 'Обычное'],
      ['LOW', 'Неважно'],
    ],
  },
  {
    key: 'status',
    label: 'Статус',
    options: [
      ['all', 'Все'],
      ['unread', 'Не прочитано'],
      ['read', 'Прочитано'],
      ['done', 'Обработано'],
    ],
  },
  {
    key: 'reply',
    label: 'Ответ',
    options: [
      ['all', 'Неважно'],
      ['yes', 'Нужен'],
      ['no', 'Не нужен'],
    ],
  },
  {
    key: 'action',
    label: 'Действие',
    options: [
      ['all', 'Неважно'],
      ['yes', 'Нужно'],
      ['no', 'Не нужно'],
    ],
  },
  {
    key: 'period',
    label: 'Период',
    options: [
      ['all', 'Всё время'],
      ['today', 'Сегодня'],
      ['week', 'Неделя'],
      ['month', 'Месяц'],
    ],
  },
];

interface Props {
  filters: Filters;
  onChange: (filters: Filters) => void;
  onReset: () => void;
}

export function FiltersPanel({ filters, onChange, onReset }: Props) {
  return (
    <div className="filters">
      {GROUPS.map((group) => (
        <div className="filters__group" key={group.key}>
          <span className="filters__label">{group.label}</span>
          <div className="filters__options">
            {group.options.map(([value, label]) => {
              const active = filters[group.key] === value;
              return (
                <button
                  key={value}
                  type="button"
                  className={`chip${active ? ' chip--on' : ''}`}
                  aria-pressed={active}
                  onClick={() => onChange({ ...filters, [group.key]: value } as Filters)}
                >
                  {label}
                </button>
              );
            })}
          </div>
        </div>
      ))}
      <button type="button" className="filters__reset" onClick={onReset}>
        Сбросить всё
      </button>
    </div>
  );
}
