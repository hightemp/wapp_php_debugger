import { expect, test, type Page } from '@playwright/test';

// The path of a user through the viewer, on logs the real library wrote (spec/fixtures/v2-library)

async function openTheItemPage(page: Page) {
  await page.goto('/');
  const feed = page.getByRole('region', { name: 'Requests' });
  await feed.getByRole('button', { name: /\/catalog\/item\?id=42/ }).click();
  await expect(
    page.getByRole('heading', { level: 2, name: /\/catalog\/item\?id=42/ }),
  ).toBeVisible();
}

test('project → request → message → variables', async ({ page }) => {
  const errors: string[] = [];
  page.on('pageerror', (error) => errors.push(error.message));
  page.on('console', (message) => message.type() === 'error' && errors.push(message.text()));

  await openTheItemPage(page);
  await expect(
    page.getByRole('navigation', { name: 'Projects' }).getByRole('button', { name: 'shop' }),
  ).toBeVisible();

  await page.getByRole('button', { name: /Loading the item/ }).click();
  await expect(page).toHaveURL(/message/);
  const context = page.getByRole('region', { name: 'Message' });
  await expect(context).toContainText('Loading the item');
  await expect(context).toContainText('item_id');
  await expect(context).toContainText('42');
  // Secrets come masked by the library
  await expect(context).toContainText('***');

  // The PHP warning of the request with its source
  await page.getByRole('button', { name: /fopen/ }).click();
  await expect(context).toContainText('failed to open stream');

  expect(errors).toEqual([]);
});

test('a link to a message opens it again', async ({ page }) => {
  await openTheItemPage(page);
  await page.getByRole('button', { name: /Loading the item/ }).click();
  const url = page.url();

  await page.goto('about:blank');
  await page.goto(url);
  await expect(page.getByRole('region', { name: 'Message' })).toContainText('item_id');
});

for (const theme of ['Light', 'Dark', 'High contrast']) {
  test(`the screens render in the ${theme} theme`, async ({ page }, testInfo) => {
    await openTheItemPage(page);
    await page.getByRole('radio', { name: theme, exact: true }).click();
    await page.getByRole('button', { name: /Loading the item/ }).click();
    await expect(page.getByRole('region', { name: 'Message' })).toContainText('item_id');

    const root = await page.evaluate(() => ({
      theme: document.documentElement.dataset.theme,
      background: getComputedStyle(document.body).backgroundColor,
      text: getComputedStyle(document.body).color,
    }));
    expect(root.theme).toBe(theme === 'High contrast' ? 'high-contrast' : theme.toLowerCase());
    expect(root.background).not.toBe(root.text);
    await testInfo.attach(`${theme}.png`, {
      body: await page.screenshot({ fullPage: true }),
      contentType: 'image/png',
    });
  });
}
