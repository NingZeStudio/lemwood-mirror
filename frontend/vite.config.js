import { defineConfig } from 'vite'
import vue from '@vitejs/plugin-vue'
import path from 'path'

// https://vite.dev/config/
// mode='production' → outDir=web/default, mode='v2' → outDir=web/default_v2
export default defineConfig(({ mode }) => {
  const themeDir = mode === 'v2' ? 'default_v2' : 'default'
  return {
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
      outDir: path.resolve(__dirname, `../web/${themeDir}`),
      emptyOutDir: true,
    },
  }
})
