// テーマの適用。<html> の .dark クラスを切り替える(index.css の
// @custom-variant がこのクラスを参照する)。"system" は OS 設定に追従する。

import type { Theme } from './api';

const media = window.matchMedia('(prefers-color-scheme: dark)');
let current: Theme = 'system';

function sync() {
  const dark = current === 'dark' || (current === 'system' && media.matches);
  document.documentElement.classList.toggle('dark', dark);
}

media.addEventListener('change', () => {
  if (current === 'system') sync();
});

export function applyTheme(theme: Theme) {
  current = theme;
  sync();
}
