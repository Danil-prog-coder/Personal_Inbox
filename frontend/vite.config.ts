import { defineConfig } from 'vitest/config';
import react from '@vitejs/plugin-react';
import { singleFile } from './build/single-file';

export default defineConfig({
  plugins: [react(), singleFile()],
  // Пути от текущей папки, а не от корня сайта: собранная страница
  // открывается и с диска, и из подкаталога.
  base: './',
  build: {
    // Всё содержимое сборки должно оказаться внутри index.html — его собирает
    // плагин singleFile. Три настройки ниже следят, чтобы наружу ничего
    // не осталось: стили одним файлом, картинки и шрифты — строкой в коде,
    // код — обычным скриптом без `import` (модуль браузер тянул бы отдельным
    // запросом, а с `file://` такой запрос запрещён).
    cssCodeSplit: false,
    assetsInlineLimit: 4 * 1024 * 1024,
    rollupOptions: {
      output: {
        format: 'iife',
        inlineDynamicImports: true,
      },
    },
  },
  server: {
    port: 5173,
    host: true,
    // Запросы к API уходят на бэкенд, поэтому сессионная cookie остаётся своей.
    // Это только для `make front`: в docker compose фронт собран в статику,
    // а /api проксирует nginx (frontend/nginx.conf).
    proxy: {
      '/api': {
        target: 'http://localhost:8000',
        changeOrigin: false,
      },
    },
  },
  test: {
    environment: 'jsdom',
    globals: true,
    setupFiles: './src/tests/setup.ts',
  },
});
