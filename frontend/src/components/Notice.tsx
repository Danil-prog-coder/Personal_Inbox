import { Icon } from './Icon';

interface Props {
  title: string;
  text: string;
  actionLabel: string;
  onAction: () => void;
}

/** Полноэкранное сообщение вместо интерфейса: приложение упало
 * (`ErrorBoundary`) или не достучалось до сервера (`App`). Пустой экран не
 * объясняет ничего, поэтому обе ситуации выглядят одинаково — заголовок,
 * причина и одно действие (решение №49, №50). */
export function Notice({ title, text, actionLabel, onAction }: Props) {
  return (
    <>
      <div className="page-bg" />
      <div className="app">
        <div className="crash" role="alert">
          <div className="crash__card glass-panel">
            <span className="crash__mark">
              <Icon name="warn" size={20} />
            </span>
            <h1 className="crash__title">{title}</h1>
            <p className="crash__text">{text}</p>
            <button type="button" className="btn-primary crash__button" onClick={onAction}>
              {actionLabel}
            </button>
          </div>
        </div>
      </div>
    </>
  );
}
