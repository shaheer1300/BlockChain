import { defineConfig, loadEnv } from 'vite'
import react from '@vitejs/plugin-react'

// Vite config — proxies /api/* to the local node so the frontend can use
// relative paths and avoid CORS in dev. Override the target with the
// VITE_API_URL env var (e.g. for nodes 2 or 3 in a multi-node demo).
export default defineConfig(({ mode }) => {
  const env = loadEnv(mode, process.cwd(), '')
  const apiTarget = env.VITE_API_URL || 'http://127.0.0.1:8001'

  return {
    plugins: [react()],
    server: {
      port: 5173,
      proxy: {
        '/api': {
          target: apiTarget,
          changeOrigin: true,
          rewrite: (path) => path.replace(/^\/api/, ''),
        },
      },
    },
  }
})
