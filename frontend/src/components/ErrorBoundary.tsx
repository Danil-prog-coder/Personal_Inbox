import { Component } from 'react';
import type { ErrorInfo, ReactNode } from 'react';
import { Notice } from './Notice';

interface Props {
  children: ReactNode;
}

interface State {
  failed: boolean;
}

/** Ловит исключение из любого места дерева. Без него React размонтирует
 * приложение целиком и остаётся пустой экран без единой подсказки
 * (решение №49). Класс, а не хук: перехват ошибок рендера в React 18
 * доступен только компоненту-классу. */
export class ErrorBoundary extends Component<Props, State> {
  state: State = { failed: false };

  static getDerivedStateFromError(): State {
    return { failed: true };
  }

  componentDidCatch(error: Error, info: ErrorInfo) {
    // Единственное место, где консоль оправдана: иначе причина падения
    // теряется вместе с деревом.
    console.error('Ошибка интерфейса:', error, info.componentStack);
  }

  render() {
    if (!this.state.failed) return this.props.children;

    return (
      <Notice
        title="Что-то пошло не так"
        text="Интерфейс не смог отрисовать этот экран. Данные не потеряны — обновите страницу."
        actionLabel="Обновить страницу"
        onAction={() => window.location.reload()}
      />
    );
  }
}
