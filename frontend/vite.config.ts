import vue from '@vitejs/plugin-vue'
import { defineConfig } from 'vitest/config'

// https://vitejs.dev/config/
export default defineConfig({
  root: `${import.meta.dirname}/src/app`,
  plugins: [vue()],
  test: {
    environment: 'happy-dom',
    include: ['../features/**/tests/*.test.ts', '../common/**/tests/*.test.ts'],
  },
})
