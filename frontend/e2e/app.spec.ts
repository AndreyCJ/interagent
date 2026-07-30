import { test, expect } from '@playwright/test'

test('app loads without errors', async ({ page }) => {
  test.fixme(true, 'E2E requires running Wails app')
  await page.goto('/')
  await expect(page.locator('#app')).toBeAttached()
})
