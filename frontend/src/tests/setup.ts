import '@testing-library/jest-dom/vitest';

// jsdom не реализует matchMedia, а на нём держится выбор носителя
// (useMediaQuery). Без заглушки любой тест, рендерящий App, падает.
// Отдаём «десктоп»: граница 900px, а окно jsdom по умолчанию 1024px.
if (!window.matchMedia) {
  window.matchMedia = (query: string) =>
    ({
      matches: true,
      media: query,
      onchange: null,
      addEventListener: () => {},
      removeEventListener: () => {},
      addListener: () => {},
      removeListener: () => {},
      dispatchEvent: () => false,
    }) as MediaQueryList;
}
