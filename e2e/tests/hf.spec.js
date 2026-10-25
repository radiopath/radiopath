import { test, expect, c, login, toast, addSite } from '../helpers.js'

test.beforeEach(({ page }) => login(page, 'hfpage'))

// the e2e stack has no ITU data set, so the prediction itself is covered by the Go tests

test('asks for two sites first', async ({ page }) => {
  await page.goto('/hf')
  await expect(page.getByRole('heading', { name: 'HF prediction' })).toBeVisible()
  await expect(page.getByText('Create at least two')).toBeVisible()
  await expect(c(page, 'button', 'Predict')).toBeDisabled()
})

test('reports fields that are no number', async ({ page }) => {
  await addSite(page, 'HF A', 47.37, 8.54)
  await addSite(page, 'HF B', 46.95, 7.45)
  await page.goto('/hf')
  await expect(c(page, 'button', 'Predict')).toBeEnabled()
  await page.locator('[name=ssn]').fill('abc')
  await page.locator('[name=frequencies_mhz]').fill('7.1, x')
  await c(page, 'button', 'Predict').click()
  await expect(toast(page, 'Sunspot number: not a number')).toBeVisible()
  await expect(toast(page, 'Frequencies: not a list of numbers')).toBeVisible()
})

test('says so when the HF model is not configured', async ({ page }) => {
  // the server's answer without the data set is covered by the Go tests; here the toast
  await page.route('**/api/hf/predictions', (route) =>
    route.fulfill({
      status: 503,
      contentType: 'application/problem+json',
      json: { title: 'Service Unavailable', status: 503, detail: 'The HF model is not configured on this server' },
    }),
  )
  await page.goto('/hf')
  await expect(c(page, 'button', 'Predict')).toBeEnabled()
  await c(page, 'button', 'Predict').click()
  await expect(toast(page, 'The HF model is not configured on this server')).toBeVisible()
})

test('shows the digital parameters only for custom digital modulation', async ({ page }) => {
  await page.goto('/hf')
  await expect(page.locator('[name=f0_hz]')).toHaveCount(0)
  await page.locator('[name=mode]').selectOption('custom')
  await expect(page.locator('[name=f0_hz]')).toHaveCount(0)
  await page.locator('[name=modulation]').selectOption('digital')
  await expect(page.locator('[name=required_sir_db]')).toHaveValue('10')
  await page.locator('[name=f0_hz]').fill('x')
  await c(page, 'button', 'Predict').click()
  await expect(toast(page, 'F0 (Hz): not a number')).toBeVisible()
})

test('takes sites, antenna and aiming from a link', async ({ page }) => {
  const sites = await (await page.request.get('/api/sites')).json()
  const ant = await page.request.post('/api/antennas', { data: { name: 'HF Sector', gain_dbi: 6, beamwidth_deg: 90, front_back_db: 20 } })
  const antenna = await ant.json()
  const made = await page.request.post('/api/links', {
    data: {
      name: 'HF Link',
      site_a_id: sites[0].id,
      site_b_id: sites[1].id,
      frequency_mhz: 28,
      polarization: 'vertical',
      tx_power_dbm: 40,
      rx_sensitivity_dbm: -100,
      tx_antenna_id: antenna.id,
      tx_azimuth_deg: 90,
    },
  })
  expect(made.status()).toBe(201)

  await page.goto('/hf')
  await expect(page.locator('[name=tx_azimuth_deg]')).toHaveCount(0)
  await page.locator('[name=from_link]').selectOption({ label: 'HF Link' })
  await expect(page.locator('[name=site_a_id]')).toHaveValue(String(sites[0].id))
  await expect(page.locator('[name=site_b_id]')).toHaveValue(String(sites[1].id))
  await expect(page.locator('[name=tx_antenna_id]')).toHaveValue(String(antenna.id))
  await expect(page.locator('[name=tx_azimuth_deg]')).toHaveValue('90')
  await expect(page.locator('[name=tx_power_dbm]')).toHaveValue('40')
  await expect(page.locator('[name=frequencies_mhz]')).toHaveValue('28')
  await expect(page.locator('[name=rx_azimuth_deg]')).toHaveCount(0)
})

// a prediction answer built by hand, so the charts and the table can be checked without the ITU data set
const point = (hour, freq, extra = {}) => ({
  month: 4, hour_utc: hour, frequency_mhz: freq, distance_km: 2000,
  muf_basic_mhz: 14, muf_10_mhz: 20 - hour / 2, muf_50_mhz: 16 - hour / 2, muf_90_mhz: 12 - hour / 2, muf_operational_mhz: 13,
  field_strength_dbuv_m: 30, rx_power_dbw: -120, noise_db: 40, snr_db: 18, snr_percentile_db: 12,
  basic_reliability_pct: hour === 6 ? 80 : 40, overall_reliability_pct: 0,
  dominant_mode: '2F2', dominant_elevation_deg: 10, dominant_delay_ms: 8, dominant_loss_db: 150, dominant_supported_pct: 90,
  dominant_reflection_height_km: 300, dominant_field_strength_dbuv_m: 30, ...extra,
})

test('draws the MUF and reliability charts', async ({ page }) => {
  await page.route('**/api/hf/predictions', (route) =>
    route.fulfill({ json: { points: [point(6, 7.1), point(6, 14.2), point(12, 7.1), point(12, 14.2, { basic_reliability_pct: -307 })] } }),
  )
  await page.goto('/hf')
  await expect(c(page, 'button', 'Predict')).toBeEnabled()
  await c(page, 'button', 'Predict').click()

  const muf = page.getByRole('img', { name: 'MUF (MHz)' })
  await expect(muf).toBeVisible()
  await expect(muf.locator('circle')).toHaveCount(2)
  await expect(muf.locator('polygon')).toHaveCount(1)
  await expect(muf.locator('line[stroke-dasharray]')).toHaveCount(2)

  // a crosshair at the nearest plotted hour with the values there, shared by both charts
  const box = await muf.boundingBox()
  await muf.hover({ position: { x: box.width * 0.6, y: box.height / 2 } })
  await expect(muf.locator('[data-crosshair]')).toHaveCount(1)
  const tip = muf.locator('..').locator('[data-tooltip]')
  await expect(tip).toContainText('12:00 UTC')
  await expect(tip).toContainText('MUF, median: 10.0 MHz')
  await expect(tip).toContainText('6.0 to 14.0 MHz')
  await expect(page.getByRole('img', { name: 'Circuit reliability (%)' }).locator('[data-crosshair]')).toHaveCount(1)
  // the grids mark the column of that hour, and hovering a grid cell moves the crosshair
  const hot = page.getByRole('region', { name: 'Reliability by hour' }).locator('td[data-hot]')
  await expect(hot).toHaveCount(2)
  await expect(hot.first()).toHaveAttribute('title', /06:00|12:00/)
  await page.mouse.move(0, 0)
  await expect(muf.locator('[data-crosshair]')).toHaveCount(0)
  await expect(hot).toHaveCount(0)
  await page.getByRole('table', { name: 'Reliability at 7.1 MHz by month and hour' }).locator('tbody td').nth(1).hover()
  await expect(hot).toHaveCount(2)
  await expect(tip).toContainText('06:00 UTC')

  // 14.2 MHz has no reliability at 12:00, so three points remain
  const rel = page.getByRole('img', { name: 'Circuit reliability (%)' })
  await expect(rel.locator('circle')).toHaveCount(3)
  await expect(rel.locator('..').locator('li', { hasText: '14.2 MHz' })).toBeVisible()

  // one grid per frequency with the month as its only row; the cells carry the colour of the map, the one without a value stays plain
  // the row is labelled by the month asked for, the current one
  const month = `${new Date().toLocaleString('en', { month: 'long', timeZone: 'UTC' })} ${new Date().getUTCFullYear()}`
  const grids = page.getByRole('region', { name: 'Reliability by hour' })
  await expect(grids.getByRole('table')).toHaveCount(2)
  const g14 = grids.getByRole('table', { name: 'Reliability at 14.2 MHz by month and hour' })
  await expect(g14.locator('thead th')).toHaveText(['14.2 MHz', '06', '12'])
  await expect(g14.locator('tbody tr')).toHaveCount(1)
  await expect(g14.locator('tbody td')).toHaveText([month, '80', '–'])
  await expect(g14.locator('tbody td[style*=background-color]')).toHaveCount(1)
  await expect(g14.locator('tbody td').nth(1)).toHaveAttribute('title', `${month}, 06:00 UTC: 80 %, SNR 18.0 dB, field 30.0 dBµV/m, MUF 13.0 MHz, 2F2`)
  await expect(page.getByText('2000 km', { exact: true })).toBeVisible()
  await expect(page.getByText('7.1 MHz at 06:00')).toBeVisible()
  await expect(page.getByText('80 % of the days')).toBeVisible()

  // a changed form marks the results stale until the next prediction
  await expect(page.getByText('The form changed')).toHaveCount(0)
  await page.locator('[name=frequencies_mhz]').fill('7.1')
  await expect(page.getByText('The form changed')).toBeVisible()
  await c(page, 'button', 'Predict').click()
  await expect(page.getByText('The form changed')).toHaveCount(0)
})

// a map answer built by hand: a frame per requested hour, one cell the model could not calculate
const mapAnswer = (body) => ({
  step_deg: 10, rows: 18, cols: 36,
  frames: body.hours_utc.map((h) => ({ hour_utc: h, reliability_pct: Array.from({ length: 648 }, (_, i) => (i === 0 ? -307 : h === 6 ? 80 : 20)) })),
})

test('shows the reliability map with an hour slider', async ({ page }) => {
  const sites = await (await page.request.get('/api/sites')).json()
  const maps = []
  await page.route('**/api/hf/predictions', (route) =>
    route.fulfill({ json: { points: [point(6, 7.1), point(6, 14.2), point(12, 7.1), point(12, 14.2)] } }),
  )
  await page.route('**/api/hf/maps', (route) => {
    const body = route.request().postDataJSON()
    maps.push(body)
    return route.fulfill({ json: mapAnswer(body) })
  })
  await page.goto('/hf')
  await expect(c(page, 'button', 'Predict')).toBeEnabled()
  await page.locator('[name=range_from]').fill('6')
  await page.locator('[name=range_to]').fill('12')
  await page.locator('[name=range_step]').fill('6')
  await c(page, 'button', 'Predict').click()

  const panel = page.getByRole('region', { name: 'Reliability map' })
  await expect(panel.getByRole('heading')).toContainText('at 7.1 MHz')
  // a flat SVG world map: a coloured rectangle per cell, the one the model could not calculate is left out; markers A and B
  const world = panel.getByRole('img', { name: 'World map' })
  await expect(world).toBeVisible()
  await expect(world.locator('rect[fill^=hsl]')).toHaveCount(647)
  await expect(world.locator('rect[fill^=hsl]').first()).toHaveAttribute('fill', 'hsl(96 75% 45%)')
  await expect(world.locator('circle')).toHaveCount(2)
  await expect(world.locator('path[data-path]')).toHaveCount(1)
  await expect(world.locator('path[data-path]')).not.toHaveAttribute('stroke-dasharray')
  // the night side of 06:00 UTC: the sun stands at 90° east
  await expect(world.locator('path[data-night]')).toHaveCount(1)
  await expect(world.locator('circle[data-sun]')).toHaveAttribute('cx', '270')
  // from site A, at the first frequency, hours in order
  expect(maps[0]).toMatchObject({ site_id: sites.find((s) => s.name === 'HF A').id, frequency_mhz: 7.1, hours_utc: [6, 12] })

  const slider = panel.locator('[name=map_hour]')
  await expect(slider).toHaveAttribute('max', '1')
  await expect(panel.getByText('06:00 UTC')).toBeVisible()
  await slider.fill('1')
  await expect(panel.getByText('12:00 UTC')).toBeVisible()
  await expect(world.locator('rect[fill^=hsl]').first()).toHaveAttribute('fill', 'hsl(24 75% 45%)')
  await expect(world.locator('circle[data-sun]')).toHaveAttribute('cx', '180')

  await panel.locator('[name=map_frequency]').selectOption('14.2')
  await expect(panel.getByRole('heading')).toContainText('at 14.2 MHz')
  expect(maps).toHaveLength(2)
  expect(maps[1].frequency_mhz).toBe(14.2)

  // one hour needs no slider
  await page.locator('[name=range_from]').fill('12')
  await c(page, 'button', 'Predict').click()
  await expect(panel.getByText('12:00 UTC')).toBeVisible()
  await expect(panel.locator('[name=map_hour]')).toHaveCount(0)
})

test('shows why the map is missing', async ({ page }) => {
  await page.route('**/api/hf/predictions', (route) => route.fulfill({ json: { points: [point(6, 7.1)] } }))
  await page.route('**/api/hf/maps', (route) =>
    route.fulfill({ status: 429, contentType: 'application/problem+json', json: { title: 'Too Many Requests', status: 429, detail: 'Too many maps, try again in a minute.' } }),
  )
  await page.goto('/hf')
  await expect(c(page, 'button', 'Predict')).toBeEnabled()
  await c(page, 'button', 'Predict').click()
  const panel = page.getByRole('region', { name: 'Reliability map' })
  await expect(panel.getByRole('alert')).toContainText('Too many maps')
  await expect(page.getByRole('img', { name: 'MUF (MHz)' })).toBeVisible()
})

test('fills the sunspot number from the server until it is typed', async ({ page }) => {
  // the value of a month is 100 plus the month; the page starts in the current month
  const ym = (offset) => {
    const d = new Date(Date.UTC(new Date().getUTCFullYear(), new Date().getUTCMonth() + offset))
    return { year: d.getUTCFullYear(), month: d.getUTCMonth() + 1, text: d.toISOString().slice(0, 7) }
  }
  const [m0, m1, m2] = [ym(0).month, ym(1).month, ym(2).month]
  const asked = []
  await page.route('**/api/hf/sunspots*', (route) => {
    const q = new URL(route.request().url()).searchParams
    asked.push(`${q.get('year')}-${q.get('month')}`)
    route.fulfill({ json: { ssn: 100 + Number(q.get('month')), source: 'predicted', reference: 'NOAA SWPC' } })
  })
  await page.goto('/hf')
  await expect(page.locator('[name=ssn]')).toHaveValue(String(100 + m0))
  await expect(page.getByTestId('ssn-source')).toContainText('predicted')

  await page.locator('[name=range_start]').fill(ym(1).text)
  await expect(page.locator('[name=ssn]')).toHaveValue(String(100 + m1))

  await page.locator('[name=ssn]').fill('120')
  await page.locator('[name=range_start]').fill(ym(2).text)
  await expect(page.getByTestId('ssn-source')).toContainText(String(100 + m2))
  await expect(page.locator('[name=ssn]')).toHaveValue('120')
  expect(asked).toContain(`${ym(1).year}-${m1}`)

  // an emptied field is filled again
  await page.locator('[name=ssn]').fill('')
  await expect(page.locator('[name=ssn]')).toHaveValue(String(100 + m2))
})

test('remembers the form in the browser', async ({ page }) => {
  await page.goto('/hf')
  await expect(page.getByText('= 100 W')).toBeVisible()
  await page.locator('[name=frequencies_mhz]').fill('3.5')
  await page.locator('[name=tx_power_dbm]').fill('43')
  await page.locator('[name=mode]').selectOption('ft8')
  await expect(page.getByText('= 20 W')).toBeVisible()
  await page.reload()
  await expect(page.locator('[name=frequencies_mhz]')).toHaveValue('3.5')
  await expect(page.locator('[name=tx_power_dbm]')).toHaveValue('43')
  await expect(page.locator('[name=mode]')).toHaveValue('ft8')
  await expect(page.locator('[name=bandwidth_hz]')).toHaveCount(0)
})

test('keeps the default sunspot number when the server has no data', async ({ page }) => {
  await page.goto('/hf')
  await expect(page.locator('[name=ssn]')).toHaveValue('60')
  await expect(page.getByTestId('ssn-source')).toHaveCount(0)
})

test('predicts every month of a range with the hours from, to and step', async ({ page }) => {
  const sent = []
  await page.route('**/api/hf/predictions', (route) => {
    const b = route.request().postDataJSON()
    sent.push(b)
    route.fulfill({ json: { points: b.hours_utc.map((h) => point(h, 7.1, { month: b.month })) } })
  })
  await page.route('**/api/hf/maps', (route) => route.fulfill({ status: 503, json: { title: 'Service Unavailable', status: 503, detail: 'no map' } }))
  await page.goto('/hf')
  await expect(c(page, 'button', 'Predict')).toBeEnabled()

  await page.locator('[name=range_start]').fill('2026-03')
  await page.locator('[name=range_end]').fill('2026-05')
  await page.locator('[name=range_from]').fill('0')
  await page.locator('[name=range_to]').fill('12')
  await page.locator('[name=range_step]').fill('6')
  await c(page, 'button', 'Predict').click()
  await expect.poll(() => sent.length).toBe(3)
  expect(sent.map((b) => [b.year, b.month])).toEqual([[2026, 3], [2026, 4], [2026, 5]])
  expect(sent[0].hours_utc).toEqual([6, 12, 24])
  await expect(page.locator('[name=period]')).toHaveValue('2026-3')
  await expect(page.locator('[name=period] option')).toHaveText(['March 2026', 'April 2026', 'May 2026'])
  await page.locator('[name=period]').selectOption({ label: 'May 2026' })

  // one grid per frequency, a row per month, a coloured cell per hour; a month is picked by its name
  const glance = page.getByRole('region', { name: 'Reliability by hour' })
  const grid = glance.getByRole('table', { name: 'Reliability at 7.1 MHz by month and hour' })
  await expect(glance.getByRole('table')).toHaveCount(1)
  await expect(grid.locator('thead th')).toHaveText(['7.1 MHz', '06', '12', '24'])
  await expect(grid.locator('tbody tr')).toHaveCount(3)
  await expect(grid.locator('tbody td[style*=background-color]')).toHaveCount(9)
  await expect(grid.getByRole('button', { name: 'May 2026' })).toHaveAttribute('aria-current', 'true')
  await grid.getByRole('button', { name: 'April 2026' }).click()
  await expect(page.locator('[name=period]')).toHaveValue('2026-4')
  await expect(grid.getByRole('button', { name: 'April 2026' })).toHaveAttribute('aria-current', 'true')
})

test('rejects a range that ends before it starts or has a bad hour window', async ({ page }) => {
  await page.goto('/hf')
  await expect(c(page, 'button', 'Predict')).toBeEnabled()
  await page.locator('[name=range_start]').fill('2026-05')
  await page.locator('[name=range_end]').fill('2026-03')
  await page.locator('[name=range_to]').fill('30')
  await page.locator('[name=range_step]').fill('0')
  await c(page, 'button', 'Predict').click()
  await expect(toast(page, 'End: before the start')).toBeVisible()
  await expect(toast(page, 'Hours: from and to must be whole hours from 0 to 23')).toBeVisible()
  await expect(toast(page, 'Step: must be a whole number of hours, 1 or more')).toBeVisible()
})

test('explains the cards with info buttons', async ({ page }) => {
  await page.route('**/api/hf/predictions', (route) => route.fulfill({ json: { points: [point(6, 7.1), point(12, 7.1)] } }))
  await page.goto('/hf')
  await expect(c(page, 'button', 'Predict')).toBeEnabled()
  await c(page, 'button', 'Predict').click()
  await page.getByRole('button', { name: 'About the Circuit reliability chart' }).click()
  await expect(page.getByRole('note')).toContainText('share of the days')
  await page.keyboard.press('Escape')
  await expect(page.getByRole('note')).toHaveCount(0)
  await page.getByRole('button', { name: 'About the reliability grid' }).click()
  await expect(page.getByRole('note')).toContainText('propagation mode')
  await page.getByRole('heading', { name: 'Circuit reliability by hour and frequency' }).click()
  await expect(page.getByRole('note')).toHaveCount(0)
})
