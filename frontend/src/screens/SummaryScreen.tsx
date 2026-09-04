import { useEffect, useState } from 'react';
import { Icon } from '../components/Icon';
import { Segmented } from '../components/Segmented';
import { api } from '../lib/api';
import { LEVEL_COLOR, LEVEL_LABEL, LEVEL_ORDER } from '../lib/levels';
import type { Summary, SummaryPeriod } from '../lib/types';

const PERIODS: { value: SummaryPeriod; label: string }[] = [
  { value: '24h', label: '24ч' },
  { value: 'week', label: 'Неделя' },
  { value: 'month', label: 'Месяц' },
];

const RANGE_LABEL: Record<SummaryPeriod, string> = {
  '24h': 'За последние 24 часа',
  week: 'За последние 7 дней',
  month: 'За последние 30 дней',
};

export function SummaryScreen({ onOpenMessage }: { onOpenMessage: (id: number) => void }) {
  const [period, setPeriod] = useState<SummaryPeriod>('24h');
  const [summary, setSummary] = useState<Summary | null>(null);
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    let cancelled = false;
    setLoading(true);
    api
      .summary(period)
      .then((data) => {
        if (!cancelled) setSummary(data);
      })
      .finally(() => {
        if (!cancelled) setLoading(false);
      });
    return () => {
      cancelled = true;
    };
  }, [period]);

  return (
    <div className="screen">
      <div className="screen-flow screen-flow--wide">
        <div className="screen-flow__column">
        <div className="head-row head-row--wrap">
          <div className="head-titles">
            <h1 className="screen-title">Сводка</h1>
            <span className="screen-subtitle">{RANGE_LABEL[period]}</span>
          </div>
          <div style={{ width: 246 }}>
            <Segmented
              options={PERIODS}
              value={period}
              onChange={setPeriod}
              ariaLabel="Период сводки"
            />
          </div>
        </div>
        {loading || !summary ? (
          <div className="loading-card">
            <span className="loading-card__spinner" aria-hidden="true" />
            <div className="head-titles">
              <span className="loading-card__title">Сводка формируется</span>
              <span className="loading-card__hint">Обычно занимает 10–20 секунд</span>
            </div>
          </div>
        ) : (
          <div className="summary-stack">
            <div className="summary-grid">
              <div className="summary-total">
                <div className="summary-total__row">
                  <span className="summary-total__number">{summary.total}</span>
                  <span className="summary-total__caption">новых сообщений</span>
                </div>
                <div className="bars bars--summary" aria-hidden="true">
                  {LEVEL_ORDER.map((level) => (
                    <div
                      key={level}
                      className="bars__segment"
                      style={{
                        flex: Math.max(summary.distribution[level] ?? 0, 0.3),
                        background: LEVEL_COLOR[level],
                        boxShadow: `0 0 8px color-mix(in srgb, ${LEVEL_COLOR[level]} 40%, transparent)`,
                      }}
                    />
                  ))}
                </div>
                <div className="summary-legend">
                  {LEVEL_ORDER.map((level) => (
                    <div className="summary-legend__item" key={level}>
                      <span
                        className="dot"
                        aria-hidden="true"
                        style={{ background: LEVEL_COLOR[level] }}
                      />
                      <span className="summary-legend__count">
                        {summary.distribution[level] ?? 0}
                      </span>
                      <span className="summary-legend__label">{LEVEL_LABEL[level]}</span>
                    </div>
                  ))}
                </div>
              </div>

              <div className="stat-card">
                <span className="stat-card__icon stat-card__icon--reply">
                  <Icon name="reply" size={19} />
                </span>
                <div className="head-titles">
                  <span className="stat-card__number">{summary.needs_reply}</span>
                  <span className="stat-card__label">требуют ответа</span>
                </div>
              </div>

              <div className="stat-card">
                <span className="stat-card__icon stat-card__icon--action">
                  <Icon name="bolt" size={19} />
                </span>
                <div className="head-titles">
                  <span className="stat-card__number">{summary.needs_action}</span>
                  <span className="stat-card__label">требуют действия</span>
                </div>
              </div>
            </div>

            <div className="highlights">
              <span className="highlights__title">Главное за период</span>
              {summary.top.length === 0 ? (
                <span className="highlights__item" style={{ color: 'var(--ink2)' }}>
                  Ничего срочного
                </span>
              ) : (
                summary.top.map((item) => (
                  <button
                    key={item.id}
                    type="button"
                    className="highlights__item"
                    onClick={() => onOpenMessage(item.id)}
                  >
                    <span
                      className="dot"
                      aria-hidden="true"
                      style={{
                        background: LEVEL_COLOR[item.level],
                        marginTop: 5,
                        boxShadow: `0 0 8px color-mix(in srgb, ${LEVEL_COLOR[item.level]} 67%, transparent)`,
                      }}
                    />
                    <span className="highlights__text">
                      {item.sender_name} — {item.subject}
                    </span>
                    <span style={{ flex: 'none', color: 'var(--ink3)', display: 'flex', marginTop: 2 }}>
                      <Icon name="chev" size={16} />
                    </span>
                  </button>
                ))
              )}
            </div>
          </div>
        )}
        </div>
      </div>
    </div>
  );
}
