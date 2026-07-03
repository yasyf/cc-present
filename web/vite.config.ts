import { defineConfig } from 'vite';
import react from '@vitejs/plugin-react';

// The daemon binds an ephemeral port and writes it to ~/.cc-present/http.json.
// For local dev, read that port and export CC_PRESENT_DEV_PORT before `bun run
// dev`; the default below is only a placeholder.
const devPort = process.env.CC_PRESENT_DEV_PORT ?? '8790';
const devTarget = `http://127.0.0.1:${devPort}`;

export default defineConfig({
  plugins: [react()],
  // Absolute asset URLs (/assets/...) so the SPA loads under deep links like /p/<ref>.
  base: '/',
  build: {
    // Output straight into the Go embed target. It lives outside this web root,
    // so emptyOutDir must be explicit.
    outDir: '../internal/web/dist',
    emptyOutDir: true,
  },
  server: {
    proxy: {
      '/api': { target: devTarget, changeOrigin: false },
      '/assets': { target: devTarget, changeOrigin: false },
      '/events': {
        target: devTarget,
        changeOrigin: false,
        // SSE must stream, never buffer.
        configure(proxy) {
          proxy.on('proxyRes', (proxyRes) => {
            proxyRes.headers['x-accel-buffering'] = 'no';
            proxyRes.headers['cache-control'] = 'no-cache';
          });
        },
      },
    },
  },
});
