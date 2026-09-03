import { defineConfig } from 'vitest/config';
import react from '@vitejs/plugin-react';

export default defineConfig({
  plugins: [react()],
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
