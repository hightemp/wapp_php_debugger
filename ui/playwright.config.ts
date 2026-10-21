import { defineConfig, devices } from '@playwright/test';

// The built viewer on the v2 fixtures: make e2e builds it first
const port = Number(process.env.E2E_PORT ?? 18840);

export default defineConfig({
  testDir: 'e2e',
  fullyParallel: false,
  // One viewer serves the whole run and keeps settings: tests go one after another
  workers: 1,
  forbidOnly: !!process.env.CI,
  retries: process.env.CI ? 1 : 0,
  reporter: process.env.CI ? [['list'], ['html', { open: 'never' }]] : 'list',
  use: {
    baseURL: `http://127.0.0.1:${port}`,
    trace: 'retain-on-failure',
    screenshot: 'only-on-failure',
  },
  // Screenshots compared with the ones in e2e/__screenshots__ are taken in the Playwright Docker
  // image only (make e2e-visual): fonts differ from machine to machine
  snapshotPathTemplate: '{testDir}/__screenshots__/{arg}{ext}',
  expect: { toHaveScreenshot: { maxDiffPixelRatio: 0.002, animations: 'disabled' } },
  projects: [
    {
      name: 'chromium',
      testIgnore: /visual\.spec\.ts/,
      use: { ...devices['Desktop Chrome'] },
    },
    {
      name: 'visual',
      testMatch: /visual\.spec\.ts/,
      use: {
        ...devices['Desktop Chrome'],
        viewport: { width: 1440, height: 900 },
        locale: 'en-US',
        timezoneId: 'UTC',
      },
    },
  ],
  webServer: {
    command: 'node e2e/serve.mjs',
    url: `http://127.0.0.1:${port}/`,
    reuseExistingServer: false,
    // SIGTERM lets serve.mjs stop the viewer and remove its temporary directory
    gracefulShutdown: { signal: 'SIGTERM', timeout: 5000 },
    timeout: 30_000,
    env: { E2E_PORT: String(port), NO_PROXY: '*', no_proxy: '*' },
  },
});
