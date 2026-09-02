import { fireEvent, render, screen } from '@testing-library/react';
import { describe, expect, it, vi } from 'vitest';
import { MessageDetails } from '../components/MessageDetails';
import type { Message } from '../lib/types';

const message: Message = {
  id: 1,
  source: 'telegram',
  external_id: '55:10',
  sender_name: 'Дима · Продукт',
  sender_addr: '@dmitry_pm',
  subject: 'Релиз 2.4 — блокер на бэкенде',
  body: 'Ребята нашли гонку в очереди задач.',
  received_at: new Date().toISOString().replace('Z', ''),
  is_read: true,
  status: 'DONE',
  level: 'HIGH',
  level_override: null,
  category: 'Разработка',
  deadline_text: 'Сегодня',
  needs_reply: true,
  needs_action: true,
  summary: 'Просят подтвердить перенос релиза.',
  external_url: 'https://t.me/dmitry_pm/10',
  analyzed_at: null,
  analysis_failed: false,
};

describe('MessageDetails', () => {
  it('показывает краткое содержание отдельным блоком', () => {
    render(<MessageDetails message={message} onClose={() => {}} onLevelChange={() => {}} />);
    expect(screen.getByText('Краткое содержание')).toBeInTheDocument();
    expect(screen.getByText('Просят подтвердить перенос релиза.')).toBeInTheDocument();
  });

  it('ведёт в оригинал сообщения', () => {
    render(<MessageDetails message={message} onClose={() => {}} onLevelChange={() => {}} />);
    const link = screen.getByRole('link', { name: /Открыть в Telegram/ });
    expect(link).toHaveAttribute('href', 'https://t.me/dmitry_pm/10');
  });

  it('без ссылки кнопку «Открыть» не рисует', () => {
    render(
      <MessageDetails
        message={{ ...message, external_url: '' }}
        onClose={() => {}}
        onLevelChange={() => {}}
      />,
    );
    expect(screen.queryByRole('link')).not.toBeInTheDocument();
  });

  it('меняет уровень и показывает подтверждение', () => {
    const onLevelChange = vi.fn();
    render(
      <MessageDetails message={message} onClose={() => {}} onLevelChange={onLevelChange} />,
    );
    fireEvent.click(screen.getByRole('tab', { name: 'Критично' }));
    expect(onLevelChange).toHaveBeenCalledWith('CRITICAL');
    expect(screen.getByText('Сохранено')).toBeInTheDocument();
  });

  it('повторный выбор того же уровня ничего не сохраняет', () => {
    const onLevelChange = vi.fn();
    render(
      <MessageDetails message={message} onClose={() => {}} onLevelChange={onLevelChange} />,
    );
    fireEvent.click(screen.getByRole('tab', { name: 'Важно' }));
    expect(onLevelChange).not.toHaveBeenCalled();
  });

  it('закрывается по Escape и по кнопке', () => {
    const onClose = vi.fn();
    render(<MessageDetails message={message} onClose={onClose} onLevelChange={() => {}} />);
    fireEvent.keyDown(window, { key: 'Escape' });
    fireEvent.click(screen.getByRole('button', { name: 'Закрыть' }));
    expect(onClose).toHaveBeenCalledTimes(2);
  });

  it('пустое краткое содержание не показывает блок', () => {
    render(
      <MessageDetails
        message={{ ...message, summary: '' }}
        onClose={() => {}}
        onLevelChange={() => {}}
      />,
    );
    expect(screen.queryByText('Краткое содержание')).not.toBeInTheDocument();
  });
});
