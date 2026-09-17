import { defineConfig } from '@playwright/test'

export default defineConfig({
  testDir: './e2e',
  timeout: 30000,
  reporter: 'list',
  expect: {
    timeout: 10000,
  },
  use: {
    baseURL: 'http://localhost:34115',
  },
  webServer: {
    command: process.env.E2E_WAILS_CMD ?? 'wails dev -tags "webkit2_41"',
    cwd: '..',
    url: 'http://localhost:34115',
    reuseExistingServer: !process.env.CI,
    timeout: 600_000,
    gracefulShutdown: { signal: 'SIGINT', timeout: 10_000 },
  },
})
