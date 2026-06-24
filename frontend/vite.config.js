import { defineConfig } from 'vite'
import vue from '@vitejs/plugin-vue'
import path from 'path'

// https://vite.dev/config/
export default defineConfig({
  base: '/',
  plugins: [
    vue(),
  ],
  resolve: {
    alias: {
      '@': path.resolve(__dirname, './src'),
    },
  },
  build: {
    outDir: path.resolve(__dirname, '../web/default'),
    emptyOutDir: true,
  },
})
