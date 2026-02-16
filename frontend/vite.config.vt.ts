import { defineConfig } from 'vite'
import vue from '@vitejs/plugin-vue'
import tailwindcss from '@tailwindcss/vite'

export default defineConfig({
  base: '/vt/',
  plugins: [vue(), tailwindcss()],
  build: {
    outDir: 'dist-vt',
    emptyOutDir: true,
    rollupOptions: {
      input: 'vt.html',
    },
  },
  server: {
    proxy: {
      '/v1/': {
        target: 'http://localhost:8075',
        changeOrigin: true,
      },
    },
  },
})
