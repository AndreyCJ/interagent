import { defineConfig } from '@playwright/test'

export default defineConfig({
  testDir: './e2e',
  timeout: 30000,
  expect: {
    timeout: 10000,
  },
  use: {
    baseURL: 'http://localhost:34115',
  },
})
