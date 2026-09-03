import { defineConfig } from 'vitest/config';
import react from '@vitejs/plugin-react';

export default defineConfig({
  plugins: [react()],
  server: {
    port: 5173,
    host: true,
    // Запросы к API уходят на бэкенд, поэтому сессионная cookie остаётся своей.
    // В docker-compose бэкенд виден по имени сервиса, а не localhost — цель
    // прокси переопределяется переменной окружения VITE_API_PROXY_TARGET.
    proxy: {
      '/api': {
        target: process.env.VITE_API_PROXY_TARGET ?? 'http://localhost:8000',
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
