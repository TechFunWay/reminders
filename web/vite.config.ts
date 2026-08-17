import { defineConfig } from 'vite'
import vue from '@vitejs/plugin-vue'

export default defineConfig({
  base: process.env.VITE_BASE_PATH || '/',
  plugins: [vue()],
  server: {
    port: 3000,
    proxy: {
      '/api': {
        target: 'http://localhost:8906',
        changeOrigin: true
      },
      '/uploads': {
        target: 'http://localhost:8906',
        changeOrigin: true
      }
    }
  }
})
