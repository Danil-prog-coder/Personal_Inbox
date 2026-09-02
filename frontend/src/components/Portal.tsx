import { useEffect, useState } from 'react';
import { createPortal } from 'react-dom';

/**
 * Листы и модальные окна рендерятся в body: иначе их перекрытие зависит
 * от разметки экрана, и притемнение фона накрывает не всё окно.
 */
export function Portal({ children }: { children: React.ReactNode }) {
  const [mounted, setMounted] = useState(false);
  useEffect(() => setMounted(true), []);
  if (!mounted) return null;
  return createPortal(children, document.body);
}
