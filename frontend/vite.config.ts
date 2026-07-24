import { defineConfig } from 'vite';
import react from '@vitejs/plugin-react';
import tailwindcss from '@tailwindcss/vite';

export default defineConfig({
  plugins: [react(), tailwindcss()],
  server: {
    // 開発時は Go バックエンド(127.0.0.1:8765)へ API をプロキシする
    proxy: {
      '/api': 'http://127.0.0.1:8765',
    },
  },
  build: {
    // go:embed で埋め込むため、Go パッケージ配下に出力する
    outDir: '../internal/web/static',
    // 古いハッシュ付きアセットがバイナリに埋め込まれないよう毎回空にする
    // (.gitkeep は package.json の build スクリプトが復元する)
    emptyOutDir: true,
  },
});
