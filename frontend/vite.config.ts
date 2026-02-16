import { defineConfig } from 'vite'
import vue from '@vitejs/plugin-vue'
import tailwindcss from '@tailwindcss/vite'

export default defineConfig({
  base: '/reviews/',
  plugins: [vue(), tailwindcss()],
  server: {
    proxy: {
      '/v1/': {
        target: 'http://localhost:8075',
        changeOrigin: true,
      },
    },
  },
})
