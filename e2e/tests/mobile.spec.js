import { devices } from '@playwright/test'
import { test, expect, login, addSite, newPage, pathname, toast, rowAction, confirmDialog } from '../helpers.js'

const user = 'mobile'

// a phone: small viewport, touch, mobile user agent (Chromium, like the other specs)
test.use({ ...devices['Pixel 7'] })

let linkID, coverageID

test.beforeAll(async ({ browser }) => {
  const page = await newPage(browser)
  await login(page, user)
  const a = await addSite(page, 'Alpha', 47.3, 9.3)
  const b = await addSite(page, 'Bravo', 47.4, 9.45)
  const link = await page.request.post('/api/links', {
    data: { name: 'Alpha to Bravo', site_a_id: a, site_b_id: b, frequency_mhz: 145.6, polarization: 'vertical', tx_power_dbm: 40, rx_sensitivity_dbm: -120 },
  })
  expect(link.status()).toBe(201)
  linkID = (await link.json()).id
  const coverage = await page.request.post('/api/coverages', {
    data: {
      name: 'Alpha 2 km', site_id: a, frequency_mhz: 433.5, polarization: 'vertical', tx_power_dbm: 30, rx_height_m: 2,
      rx_sensitivity_dbm: -110, range_km: 2, resolution_m: 100,
    },
  })
  expect(coverage.status()).toBe(201)
  coverageID = (await coverage.json()).id
  await page.close()
})
test.beforeEach(({ page }) => login(page, user))

test('navigates with the tab bar', async ({ page }) => {
  await page.goto('/sites')
  const tabs = page.getByRole('navigation', { name: 'Sections' })
  await expect(tabs).toHaveCount(1)
  await tabs.getByRole('link', { name: 'Links' }).click()
  await expect.poll(() => pathname(page)).toBe('/links')
  await expect(tabs.getByRole('link', { name: 'Links' })).toHaveAttribute('aria-current', 'page')
  await tabs.getByRole('link', { name: 'Antennas' }).click()
  await expect.poll(() => pathname(page)).toBe('/antennas')
})

test('keeps the tab bar above the map', async ({ page }) => {
  await page.goto('/sites')
  await expect(page.locator('.leaflet-container')).toBeVisible()
  // scroll until the top of the map lies under the tab bar, then ask what the bar's bottom edge shows
  const onTop = await page.evaluate(() => {
    const map = document.querySelector('.map')
    window.scrollTo(0, map.getBoundingClientRect().top + window.scrollY - window.innerHeight + 30)
    const hit = document.elementFromPoint(20, window.innerHeight - 10)
    return !!hit?.closest('nav[aria-label="Sections"]')
  })
  expect(onTop).toBe(true)
})

test('fits the screen without scrolling sideways', async ({ page }) => {
  for (const path of ['/sites', '/links', '/coverages', '/antennas', '/hf', '/account', `/links/${linkID}`, `/coverages/${coverageID}`, '/sites/new', '/account/security', '/account/tokens', '/account/api-reference']) {
    await page.goto(path)
    await expect(page.locator('main h1')).toBeVisible()
    const overflow = await page.evaluate(() => document.documentElement.scrollWidth - window.innerWidth)
    expect(overflow, path).toBeLessThanOrEqual(0)
  }
})

test('fits the guest pages to the screen', async ({ page }) => {
  await page.context().clearCookies()
  for (const path of ['/login', '/register', '/forgot']) {
    await page.goto(path)
    await expect(page.locator('main h1')).toBeVisible()
    const overflow = await page.evaluate(() => document.documentElement.scrollWidth - window.innerWidth)
    expect(overflow, path).toBeLessThanOrEqual(0)
  }
})

test('deletes from the row menu', async ({ page }) => {
  await page.request.post('/api/sites', { data: { name: 'Throwaway', lat: 47.2, lon: 9.2, antenna_height_m: 10 } })
  await page.goto('/sites')
  await rowAction(page, 'Throwaway', 'Delete')
  await confirmDialog(page)
  await expect(toast(page, 'Site Throwaway deleted')).toBeVisible()
  await expect(page.locator('tr', { hasText: 'Throwaway' })).toHaveCount(0)
})
