import { useEffect } from 'react';
import { Icon } from './Icon';

interface Props {
  text: string;
  onHide: () => void;
}

/** Плашка подтверждения. Живёт 2.2 секунды, как в референсе. */
export function Toast({ text, onHide }: Props) {
  useEffect(() => {
    const timer = window.setTimeout(onHide, 2200);
    return () => window.clearTimeout(timer);
  }, [text, onHide]);

  return (
    <div className="toast-layer">
      <div className="toast" role="status">
        <span className="toast__icon">
          <Icon name="check" size={17} />
        </span>
        <span className="toast__text">{text}</span>
      </div>
    </div>
  );
}
