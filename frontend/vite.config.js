import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'
import tailwindcss from '@tailwindcss/vite'

const backend = 'http://localhost:8080'

export default defineConfig({
  plugins: [react(), tailwindcss()],
  server: {
    host: true,
    proxy: {
      '/api':     { target: backend, changeOrigin: true, ws: true },
      '/media':   { target: backend, changeOrigin: true },
      '/raw':     { target: backend, changeOrigin: true },
      '/answers': { target: backend, changeOrigin: true },
    },
  },
})
