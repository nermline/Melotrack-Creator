import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'
import tailwindcss from '@tailwindcss/vite'

// Бекенд (Go) у dev слухає :8080. Vite проксує туди API/медіа/WebSocket, тож
// фронтенд звертається відносними шляхами й працює як локально, так і з інших
// пристроїв мережі (host: true відкриває dev-сервер на всіх інтерфейсах).
const backend = 'http://localhost:8080'

export default defineConfig({
  plugins: [react(), tailwindcss()],
  server: {
    host: true, // доступ із LAN: http://IP-машини:5173
    proxy: {
      '/api':     { target: backend, changeOrigin: true, ws: true },
      '/login':   { target: backend, changeOrigin: true },
      '/refresh': { target: backend, changeOrigin: true },
      '/media':   { target: backend, changeOrigin: true },
      '/raw':     { target: backend, changeOrigin: true },
      '/answers': { target: backend, changeOrigin: true },
      '/play':    { target: backend, changeOrigin: true },
    },
  },
})
