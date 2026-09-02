/** Сегментированный контрол с линзой — один компонент на четыре места
 * (таб-бар, боковая навигация, период сводки, уровень важности).
 * Ширина линзы считается формулой, а не хардкодится. */

interface Option<T extends string> {
  value: T;
  label: string;
}

interface Props<T extends string> {
  options: Option<T>[];
  value: T;
  onChange: (value: T) => void;
  variant?: 'pill' | 'inset';
  ariaLabel?: string;
}

export function Segmented<T extends string>({
  options,
  value,
  onChange,
  variant = 'pill',
  ariaLabel,
}: Props<T>) {
  const index = Math.max(0, options.findIndex((option) => option.value === value));
  return (
    <div
      className={`segmented${variant === 'inset' ? ' segmented--inset' : ''}`}
      role="tablist"
      aria-label={ariaLabel}
      style={
        {
          '--seg-count': options.length,
          '--seg-index': index,
        } as React.CSSProperties
      }
    >
      <div className="lens" aria-hidden="true" />
      {options.map((option) => (
        <button
          key={option.value}
          type="button"
          role="tab"
          aria-selected={option.value === value}
          className={`segmented__button${option.value === value ? ' segmented__button--on' : ''}`}
          onClick={() => onChange(option.value)}
        >
          {option.label}
        </button>
      ))}
    </div>
  );
}
