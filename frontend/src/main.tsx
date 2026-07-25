import { StrictMode } from 'react';
import { createRoot } from 'react-dom/client';
// オフラインの Windows ネイティブ実行でも同じ書体になるよう、
// Design.pen の 3 書体はバンドルする(Google Fonts CDN は使わない)
import '@fontsource-variable/inter';
import '@fontsource-variable/geist';
import '@fontsource/ibm-plex-mono/400.css';
import '@fontsource/ibm-plex-mono/500.css';
import '@fontsource/ibm-plex-mono/600.css';
import App from './App';
import './index.css';

createRoot(document.getElementById('root')!).render(
  <StrictMode>
    <App />
  </StrictMode>,
);
