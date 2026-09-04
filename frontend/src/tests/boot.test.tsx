import { render, screen } from '@testing-library/react';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { ErrorBoundary } from '../components/ErrorBoundary';
import { singleFile } from '../../build/single-file';
// Разметка читается как текст: тест сторожит сам файл, а не его копию.
import indexHtml from '../../index.html?raw';

/** Три рубежа против пустого экрана: заставка в разметке, сборка одним
 * файлом и перехват ошибки рендера. Тесты сторожат каждый (решение №49). */

describe('ErrorBoundary', () => {
  // React печатает пойманную ошибку сам — иначе вывод тестов не читается.
  beforeEach(() => vi.spyOn(console, 'error').mockImplementation(() => {}));
  afterEach(() => vi.restoreAllMocks());

  function Boom(): JSX.Element {
    throw new Error('падение внутри дерева');
  }

  it('пропускает содержимое, пока ошибки нет', () => {
    render(
      <ErrorBoundary>
        <p>Лента</p>
      </ErrorBoundary>,
    );
    expect(screen.getByText('Лента')).toBeInTheDocument();
  });

  it('показывает экран вместо пустоты, когда компонент падает', () => {
    render(
      <ErrorBoundary>
        <Boom />
      </ErrorBoundary>,
    );
    expect(screen.getByRole('alert')).toBeInTheDocument();
    expect(screen.getByText('Что-то пошло не так')).toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Обновить страницу' })).toBeInTheDocument();
  });

  it('падение одной ветки не уносит соседнюю', () => {
    render(
      <>
        <ErrorBoundary>
          <Boom />
        </ErrorBoundary>
        <ErrorBoundary>
          <p>Сводка</p>
        </ErrorBoundary>
      </>,
    );
    expect(screen.getByText('Что-то пошло не так')).toBeInTheDocument();
    expect(screen.getByText('Сводка')).toBeInTheDocument();
  });
});

describe('index.html', () => {
  const html = indexHtml;

  it('не просит иконку отдельным файлом: её неоткуда взять до сборки', () => {
    expect(html).not.toMatch(/<link[^>]+rel="icon"[^>]+href="\.?\/[^"]*\.svg"/);
    expect(html).toContain('data:image/svg+xml');
  });

  it('несёт заставку с текстом для обоих случаев', () => {
    expect(html).toContain('id="boot"');
    expect(html).toContain('data-when="source"');
    expect(html).toContain('data-when="built"');
  });

  it('убирает заставку, когда приложение всё-таки встало', () => {
    // Без наблюдателя заставка накрыла бы экран ErrorBoundary: тот рисуется
    // уже после того, как ошибка вылетела наружу.
    expect(html).toContain('MutationObserver');
    expect(html).toContain("boot.dataset.visible = 'no'");
  });
});

describe('плагин single-file', () => {
  /** Дёргает generateBundle плагина в обход rollup. */
  function run(bundle: Record<string, unknown>) {
    const hook = singleFile().generateBundle;
    const handler = typeof hook === 'function' ? hook : hook?.handler;
    handler?.call(
      {} as never,
      {} as never,
      bundle as never,
      false,
    );
    const page = bundle['index.html'] as { source?: string } | undefined;
    return String(page?.source ?? '');
  }

  /** Минимальный бандл в форме, которую отдаёт rollup. */
  function makeBundle(code = 'console.log(1)', css = 'body{color:red}') {
    return {
      'index.html': { type: 'asset' as const, source: page },
      'assets/index-abc123.js': { type: 'chunk' as const, code },
      'assets/index-def456.css': { type: 'asset' as const, source: css },
    };
  }

  const page = [
    '<html><head>',
    '<script type="module" crossorigin src="./assets/index-abc123.js"></script>',
    '<link rel="stylesheet" crossorigin href="./assets/index-def456.css">',
    '</head><body><div id="root"></div></body></html>',
  ].join('\n');

  it('вшивает код и стили, не оставляя внешних ссылок', () => {
    const bundle = makeBundle();
    const html = run(bundle);
    expect(html).toContain('console.log(1)');
    expect(html).toContain('body{color:red}');
    expect(html).not.toContain('./assets/');
    expect(html).not.toContain('type="module"');
    // Отдельные файлы удалены из сборки — иначе рядом лежал бы мёртвый груз.
    expect(Object.keys(bundle)).toEqual(['index.html']);
  });

  it('ставит код в конец body: в <head> ему нечего монтировать', () => {
    const html = run(makeBundle());
    expect(html.indexOf('<div id="root">')).toBeLessThan(html.indexOf('console.log(1)'));
    expect(html).toMatch(/console\.log\(1\)[\s\S]*<\/body>/);
  });

  it('не толкует $-последовательности в коде как ссылки на совпадение', () => {
    const html = run(makeBundle('var s = "$&$\'$`";', 'body{content:"$&"}'));
    expect(html).toContain('var s = "$&$\'$`";');
    expect(html).toContain('body{content:"$&"}');
  });

  it('молчит, когда index.html в сборке нет', () => {
    const bundle = { 'assets/index-abc123.js': { type: 'chunk' as const, code: 'x' } };
    expect(() => run(bundle)).not.toThrow();
    expect(Object.keys(bundle)).toEqual(['assets/index-abc123.js']);
  });
});
