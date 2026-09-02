import { fireEvent, render, screen } from '@testing-library/react';
import { describe, expect, it, vi } from 'vitest';
import { FiltersPanel } from '../components/FiltersPanel';
import { MessageCard } from '../components/MessageCard';
import { Segmented } from '../components/Segmented';
import { SourceCardItem } from '../components/SourceCardItem';
import { EMPTY_FILTERS } from '../lib/types';
import type { Message, SourceCard } from '../lib/types';

function makeMessage(overrides: Partial<Message> = {}): Message {
  return {
    id: 1,
    source: 'gmail',
    external_id: 'ext-1',
    sender_name: 'Анна Ковалёва',
    sender_addr: 'a.kovaleva@northline.io',
    subject: 'Договор Northline — правки до конца дня',
    body: 'Юристы вернули версию с комментариями.',
    received_at: new Date().toISOString().replace('Z', ''),
    is_read: false,
    status: 'DONE',
    level: 'CRITICAL',
    level_override: null,
    category: 'Юридическое',
    deadline_text: 'Сегодня, 18:00',
    needs_reply: true,
    needs_action: true,
    summary: 'Нужно решение по двум пунктам.',
    external_url: 'https://mail.google.com/',
    analyzed_at: null,
    analysis_failed: false,
    ...overrides,
  };
}

function makeCard(overrides: Partial<SourceCard> = {}): SourceCard {
  return {
    kind: 'gmail',
    state: 'active',
    account: 'me@northline.io',
    last_sync_at: null,
    total: 11,
    unread: 3,
    distribution: { CRITICAL: 1, HIGH: 3, NORMAL: 4, LOW: 3 },
    urgent: { id: 1, sender_name: 'Анна Ковалёва', subject: 'Договор', level: 'CRITICAL' },
    ...overrides,
  };
}

describe('Segmented', () => {
  const options = [
    { value: '24h', label: '24ч' },
    { value: 'week', label: 'Неделя' },
    { value: 'month', label: 'Месяц' },
  ];

  it('ширина линзы считается по числу сегментов, а не хардкодится', () => {
    const { container } = render(
      <Segmented options={options} value="week" onChange={() => {}} />,
    );
    const control = container.querySelector('.segmented') as HTMLElement;
    expect(control.style.getPropertyValue('--seg-count')).toBe('3');
    expect(control.style.getPropertyValue('--seg-index')).toBe('1');
    expect(container.querySelector('.lens')).toBeInTheDocument();
  });

  it('сообщает о выборе и помечает активный сегмент', () => {
    const onChange = vi.fn();
    render(<Segmented options={options} value="24h" onChange={onChange} />);
    expect(screen.getByRole('tab', { name: '24ч' })).toHaveAttribute('aria-selected', 'true');
    fireEvent.click(screen.getByRole('tab', { name: 'Месяц' }));
    expect(onChange).toHaveBeenCalledWith('month');
  });
});

describe('MessageCard', () => {
  it('показывает уровень цветом, иконкой и подписью', () => {
    const { container } = render(
      <MessageCard message={makeMessage()} isNew={false} onOpen={() => {}} />,
    );
    expect(screen.getByText('Критично')).toBeInTheDocument();
    expect(container.querySelector('.level-chip svg')).toBeInTheDocument();
    expect(container.querySelector('.msg-card__edge')).toBeInTheDocument();
  });

  it('в обработке показывает индикатор вместо оценки', () => {
    render(
      <MessageCard
        message={makeMessage({ status: 'PROCESSING', category: '—', deadline_text: '' })}
        isNew
        onOpen={() => {}}
      />,
    );
    expect(screen.getByText('Определяем важность…')).toBeInTheDocument();
    expect(screen.queryByText('Критично')).not.toBeInTheDocument();
  });

  it('пустые категорию и срок не рисует', () => {
    render(
      <MessageCard
        message={makeMessage({ category: '—', deadline_text: '' })}
        isNew={false}
        onOpen={() => {}}
      />,
    );
    expect(screen.queryByText('—')).not.toBeInTheDocument();
    expect(screen.queryByText('Сегодня, 18:00')).not.toBeInTheDocument();
  });

  it('сообщает, когда оценка недоступна', () => {
    render(
      <MessageCard
        message={makeMessage({ analysis_failed: true })}
        isNew={false}
        onOpen={() => {}}
      />,
    );
    expect(screen.getByText('Оценка недоступна')).toBeInTheDocument();
  });

  it('новую карточку подсвечивает', () => {
    const { container } = render(
      <MessageCard message={makeMessage()} isNew onOpen={() => {}} />,
    );
    expect(container.querySelector('.msg-card--new')).toBeInTheDocument();
  });

  it('по клику отдаёт сообщение наверх', () => {
    const onOpen = vi.fn();
    render(<MessageCard message={makeMessage()} isNew={false} onOpen={onOpen} />);
    fireEvent.click(screen.getByRole('button'));
    expect(onOpen).toHaveBeenCalledWith(expect.objectContaining({ id: 1 }));
  });
});

describe('SourceCardItem', () => {
  it('рисует все четыре сегмента полосы, включая нулевые', () => {
    const { container } = render(
      <SourceCardItem
        card={makeCard({ distribution: { CRITICAL: 2, HIGH: 0, NORMAL: 0, LOW: 1 } })}
        onOpen={() => {}}
      />,
    );
    const segments = container.querySelectorAll('.bars__segment');
    expect(segments).toHaveLength(4);
    // Нулевой уровень не схлопывается: flex не меньше 0.3.
    expect((segments[1] as HTMLElement).style.flexGrow).toBe('0.3');
  });

  it('показывает самое срочное сообщение строкой', () => {
    render(<SourceCardItem card={makeCard()} onOpen={() => {}} />);
    expect(screen.getByText('Анна Ковалёва — Договор')).toBeInTheDocument();
  });

  it('без срочного пишет «Ничего срочного»', () => {
    render(<SourceCardItem card={makeCard({ urgent: null })} onOpen={() => {}} />);
    expect(screen.getByText('Ничего срочного')).toBeInTheDocument();
  });

  it('при reauth предупреждает прямо на карточке', () => {
    render(<SourceCardItem card={makeCard({ state: 'reauth' })} onOpen={() => {}} />);
    expect(
      screen.getByText('Нужна повторная авторизация — новые сообщения не поступают'),
    ).toBeInTheDocument();
  });

  it('счётчик непрочитанных скрыт при нуле', () => {
    const { container } = render(
      <SourceCardItem card={makeCard({ unread: 0 })} onOpen={() => {}} />,
    );
    expect(container.querySelector('.source-card__unread')).not.toBeInTheDocument();
  });
});

describe('FiltersPanel', () => {
  it('показывает пять групп фильтров', () => {
    render(<FiltersPanel filters={EMPTY_FILTERS} onChange={() => {}} onReset={() => {}} />);
    for (const label of ['Важность', 'Статус', 'Ответ', 'Действие', 'Период']) {
      expect(screen.getByText(label)).toBeInTheDocument();
    }
  });

  it('меняет один фильтр, не трогая остальные', () => {
    const onChange = vi.fn();
    render(<FiltersPanel filters={EMPTY_FILTERS} onChange={onChange} onReset={() => {}} />);
    fireEvent.click(screen.getByRole('button', { name: 'Критично' }));
    expect(onChange).toHaveBeenCalledWith({ ...EMPTY_FILTERS, level: 'CRITICAL' });
  });

  it('кнопка «Сбросить всё» зовёт сброс', () => {
    const onReset = vi.fn();
    render(<FiltersPanel filters={EMPTY_FILTERS} onChange={() => {}} onReset={onReset} />);
    fireEvent.click(screen.getByRole('button', { name: 'Сбросить всё' }));
    expect(onReset).toHaveBeenCalled();
  });
});
