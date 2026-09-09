import { test, expect, type Page } from '@playwright/test'

// Wails injects its own dev-runner runtime (`/wails/ipc.js`) into browser
// sessions, including a loading spinner that throws a benign pageerror in this
// Wails version. App errors always come from the app bundle, so we only fail
// on pageerrors whose stack does NOT originate in the injected runtime.
function collectUnexpectedErrors(page: Page, errors: string[]) {
  page.on('pageerror', error => {
    const stack = error.stack ?? error.message
    if (!stack.includes('/wails/ipc.js')) errors.push(`pageerror: ${stack}`)
  })
  page.on('console', message => {
    if (message.type() === 'error') errors.push(`console.error: ${message.text()}`)
  })
}

test('app starts and renders the UI', async ({ page }) => {
  const errors: string[] = []
  collectUnexpectedErrors(page, errors)

  await page.goto('/')

  await expect(page.locator('#app')).toBeAttached()
  await expect(page.locator('#app')).not.toBeEmpty()
  await expect(page.locator('.overlay')).toBeVisible()

  await expect(page.locator('.mic-button')).toBeVisible()
  await expect(page.locator('.mic-button')).toHaveText('Listen')

  const controls = page.locator('.controls')
  await expect(controls.getByRole('button', { name: /^Mode:/ })).toBeVisible()
  await expect(controls.getByRole('button', { name: 'Show' })).toBeVisible()
  await expect(controls.getByRole('button', { name: 'Hide' })).toBeVisible()
  await expect(controls.getByRole('button', { name: 'Settings' })).toBeVisible()

  await expect(page.locator('.chat-panel')).toBeVisible()

  expect(errors).toEqual([])
})

test('backend is reachable over the dev IPC bridge', async ({ page }) => {
  await page.goto('/')

  await expect
    .poll(() =>
      page.evaluate(() => {
        const go = (
          window as unknown as { go?: { main?: { App?: { GetVersion?: () => Promise<string> } } } }
        ).go
        return go?.main?.App?.GetVersion ? go.main.App.GetVersion() : null
      }),
    )
    .toBe('0.1.0')
})
