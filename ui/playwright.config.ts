import { defineConfig, devices } from '@playwright/test';

// The built viewer on the v2 fixtures: make e2e builds it first
const port = Number(process.env.E2E_PORT ?? 18840);

export default defineConfig({
  testDir: 'e2e',
  fullyParallel: false,
  forbidOnly: !!process.env.CI,
  retries: process.env.CI ? 1 : 0,
  reporter: process.env.CI ? [['list'], ['html', { open: 'never' }]] : 'list',
  use: {
    baseURL: `http://127.0.0.1:${port}`,
    trace: 'retain-on-failure',
    screenshot: 'only-on-failure',
  },
  projects: [{ name: 'chromium', use: { ...devices['Desktop Chrome'] } }],
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
