import vue from '@vitejs/plugin-vue'
import { resolve } from 'node:path'
import { defineConfig } from 'vitest/config'

// https://vitejs.dev/config/
export default defineConfig({
  root: resolve(__dirname, 'src/app'),
  plugins: [vue()],
  test: {
    environment: 'happy-dom',
    include: ['../features/**/tests/*.test.ts', '../common/**/tests/*.test.ts'],
  },
})
