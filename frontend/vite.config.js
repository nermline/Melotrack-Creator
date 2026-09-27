import { defineConfig } from 'vite';
import react from '@vitejs/plugin-react';
import tailwindcss from '@tailwindcss/vite';

// In development the Go server runs on :8080 and Vite proxies API, files and WebSockets to it.
const backend = process.env.BACKEND || 'http://localhost:8080';

export default defineConfig({
    plugins: [react(), tailwindcss()],
    server: {
        host: true,
        proxy: {
            '/api': { target: backend, ws: true },
            '/files': { target: backend },
        },
    },
    build: {
        chunkSizeWarningLimit: 900,
    },
});
