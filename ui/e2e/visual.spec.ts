import { expect, test, type Page } from '@playwright/test';
import { resetSettings } from './settings';

// Screens compared with e2e/__screenshots__: run in the Playwright Docker image (make e2e-visual), and
// make e2e-visual-update after a change of the look that is meant

const themes = ['light', 'dark', 'high-contrast'] as const;

// The version comes from git describe: it changes with every commit
const mask = (page: Page) => [page.locator('.version')];

for (const theme of themes) {
  test(`the viewer in the ${theme} theme`, async ({ page, request }) => {
    await resetSettings(request, { theme });
    await page.goto('/');
    await page
      .getByRole('region', { name: 'Requests' })
      .getByRole('button', { name: /\/catalog\/item\?id=42/ })
      .click();
    await page.getByRole('button', { name: /Loading the item/ }).click();
    await expect(page.getByRole('region', { name: 'Message' })).toContainText('item_id');
    await expect(page).toHaveScreenshot(`viewer-${theme}.png`, { mask: mask(page) });
  });
}

test('the component showcase', async ({ page, request }) => {
  await resetSettings(request, { theme: 'light' });
  await page.goto('/#/dev/components');
  await expect(page.getByRole('region', { name: /^Theme / })).toHaveCount(3);
  await expect(page).toHaveScreenshot('showcase.png', { fullPage: true });
});

test('the theme editor', async ({ page, request }) => {
  await resetSettings(request, { theme: 'dark', density: 'compact', accent_color: '#d14d00' });
  await page.goto('/');
  await page.getByRole('button', { name: 'Appearance' }).click();
  await expect(page.getByRole('dialog', { name: 'Appearance' })).toHaveScreenshot('appearance.png');
  await page.getByRole('button', { name: 'Theme editor…' }).click();
  await expect(page).toHaveScreenshot('theme-editor.png', { mask: mask(page) });
});

test('the help', async ({ page, request }) => {
  await resetSettings(request, { theme: 'light' });
  await page.goto('/');
  await page.getByRole('button', { name: 'Help' }).click();
  const help = page.getByRole('dialog', { name: 'Help' });
  await expect(help).toHaveScreenshot('help-overview.png');
  await help.getByRole('button', { name: /Connect the library/ }).click();
  await expect(help).toHaveScreenshot('help-library.png');
});

test('the viewer in a narrower window', async ({ page, request }) => {
  await resetSettings(request, { theme: 'light' });
  await page.setViewportSize({ width: 1100, height: 800 });
  await page.goto('/');
  await page
    .getByRole('region', { name: 'Requests' })
    .getByRole('button', { name: /\/catalog\/item\?id=42/ })
    .click();
  await page.getByRole('button', { name: /Loading the item/ }).click();
  await expect(page.getByRole('region', { name: 'Message' })).toContainText('item_id');
  await expect(page).toHaveScreenshot('viewer-narrow.png', { mask: mask(page) });
});
