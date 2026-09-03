import type { Plugin } from 'vite';

/** Складывает стили и код внутрь index.html, чтобы сборка была одним файлом.
 *
 * Иначе собранная страница тянет `./assets/*.js` отдельными запросами, а
 * браузер отказывается грузить их с `file://` — модуль с другого origin
 * запрещён политикой CORS, и вместо приложения остаётся пустой экран.
 * Один файл открывается откуда угодно: с диска, из любой папки, из nginx.
 * Плата — ассеты больше не кэшируются по отдельности; для проекта такого
 * размера это дешевле, чем сломанное открытие (решение №49). */
export function singleFile(): Plugin {
  return {
    name: 'personal-inbox-single-file',
    enforce: 'post',
    apply: 'build',
    generateBundle(_options, bundle) {
      const page = bundle['index.html'];
      if (!page || page.type !== 'asset') return;

      let html = String(page.source);
      const scripts: string[] = [];

      for (const [name, item] of Object.entries(bundle)) {
        // Имя файла в разметке — с путём и хешем, ищем по базовому имени.
        const file = name.split('/').pop();
        if (!file) continue;
        const quoted = file.replace(/[.*+?^${}()|[\]\\]/g, '\\$&');

        if (item.type === 'chunk' && name.endsWith('.js')) {
          html = html.replace(new RegExp(`<script[^>]*src="[^"]*${quoted}"[^>]*></script>`), '');
          scripts.push(item.code);
          delete bundle[name];
        }

        if (item.type === 'asset' && name.endsWith('.css')) {
          // Замена функцией, а не строкой: в содержимом встречаются `$&` и
          // `$'`, которые иначе будут раскрыты как ссылки на совпадение.
          html = html.replace(
            new RegExp(`<link[^>]*href="[^"]*${quoted}"[^>]*>`),
            () => `<style>\n${String(item.source)}</style>`,
          );
          delete bundle[name];
        }
      }

      // Код уезжает в самый конец body. Vite ставит тег в <head>, и это верно
      // для `type="module"` — тот ждёт разбора документа. Обычный скрипт
      // выполняется на месте, то есть до появления <div id="root">, и первым
      // же действием получает null вместо контейнера.
      if (scripts.length > 0) {
        const code = scripts.map((script) => `<script>\n${script}</script>`).join('\n');
        html = html.replace('</body>', () => `${code}\n</body>`);
      }

      page.source = html;
    },
  };
}
