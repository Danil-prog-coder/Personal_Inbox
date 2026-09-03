import { Component } from 'react';
import type { ErrorInfo, ReactNode } from 'react';
import { Icon } from './Icon';

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
      <>
        <div className="page-bg" />
        <div className="app">
          <div className="crash" role="alert">
            <div className="crash__card glass-panel">
              <span className="crash__mark">
                <Icon name="warn" size={20} />
              </span>
              <h1 className="crash__title">Что-то пошло не так</h1>
              <p className="crash__text">
                Интерфейс не смог отрисовать этот экран. Данные не потеряны — обновите страницу.
              </p>
              <button
                type="button"
                className="btn-primary crash__button"
                onClick={() => window.location.reload()}
              >
                Обновить страницу
              </button>
            </div>
          </div>
        </div>
      </>
    );
  }
}
