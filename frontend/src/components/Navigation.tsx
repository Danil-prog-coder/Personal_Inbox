import { Icon, LogoIcon } from './Icon';
import type { IconName } from './Icon';

export type Tab = 'feed' | 'summary' | 'connections' | 'settings';

export const TABS: { key: Tab; label: string; icon: IconName; path: string }[] = [
  { key: 'feed', label: 'Лента', icon: 'feed', path: '/feed' },
  { key: 'summary', label: 'Сводка', icon: 'summary', path: '/summary' },
  { key: 'connections', label: 'Источники', icon: 'connections', path: '/connections' },
  { key: 'settings', label: 'Настройки', icon: 'settings', path: '/settings' },
];

interface Props {
  active: Tab;
  unread: number;
  reauth: number;
  email: string;
  onNavigate: (tab: Tab) => void;
}

function badgeFor(tab: Tab, unread: number, reauth: number): number {
  if (tab === 'feed') return unread;
  if (tab === 'connections') return reauth;
  return 0;
}

/** Боковая панель 252px — десктоп. */
export function Rail({ active, unread, reauth, email, onNavigate }: Props) {
  const index = TABS.findIndex((tab) => tab.key === active);
  return (
    <nav className="rail" aria-label="Разделы">
      <div className="rail__brand">
        <span className="rail__logo">
          <LogoIcon size={15} />
        </span>
        <span className="rail__name">Personal Inbox</span>
      </div>
      <div className="rail__nav" style={{ '--seg-index': Math.max(index, 0) } as React.CSSProperties}>
        <div className="rail__lens" aria-hidden="true" />
        {TABS.map((tab) => {
          const badge = badgeFor(tab.key, unread, reauth);
          return (
            <button
              key={tab.key}
              type="button"
              className={`rail__item${tab.key === active ? ' rail__item--on' : ''}`}
              aria-current={tab.key === active ? 'page' : undefined}
              onClick={() => onNavigate(tab.key)}
            >
              <span className="rail__icon">
                <Icon name={tab.icon} size={17} />
              </span>
              <span className="rail__label">{tab.label}</span>
              {badge > 0 && (
                <span className={`badge${tab.key === 'connections' ? ' badge--warn' : ''}`}>
                  {badge}
                </span>
              )}
            </button>
          );
        })}
      </div>
      <div className="rail__spacer" />
      <div className="rail__user">
        <span className="rail__avatar">{email.slice(0, 1).toUpperCase()}</span>
        <div className="head-titles">
          <span className="msg-card__from ellipsis">{email}</span>
          <span className="msg-card__addr ellipsis">Аккаунт</span>
        </div>
      </div>
    </nav>
  );
}

/** Нижний таб-бар — телефон. */
export function TabBar({ active, unread, reauth, onNavigate }: Omit<Props, 'email'>) {
  const index = TABS.findIndex((tab) => tab.key === active);
  return (
    <div className="tabbar">
      <nav
        className="tabbar__inner"
        aria-label="Разделы"
        style={
          { '--seg-count': TABS.length, '--seg-index': Math.max(index, 0) } as React.CSSProperties
        }
      >
        <div className="lens" aria-hidden="true" />
        {TABS.map((tab) => {
          const badge = badgeFor(tab.key, unread, reauth);
          return (
            <button
              key={tab.key}
              type="button"
              className={`tabbar__button${tab.key === active ? ' tabbar__button--on' : ''}`}
              aria-current={tab.key === active ? 'page' : undefined}
              onClick={() => onNavigate(tab.key)}
            >
              <span style={{ display: 'flex' }}>
                <Icon name={tab.icon} size={21} />
              </span>
              <span className="tabbar__label">{tab.label}</span>
              {badge > 0 && (
                <span
                  className={`badge tabbar__badge${
                    tab.key === 'connections' ? ' badge--warn' : ''
                  }`}
                >
                  {badge}
                </span>
              )}
            </button>
          );
        })}
      </nav>
    </div>
  );
}
