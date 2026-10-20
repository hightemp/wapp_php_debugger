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

test('a request with thousands of messages scrolls whole', async ({ page }) => {
  await page.goto('/');
  await page
    .getByRole('region', { name: 'Requests' })
    .getByRole('button', { name: /3000 messages/ })
    .click();
  const list = page.getByRole('list', { name: 'Messages' });
  await expect(list.getByRole('listitem').first()).toHaveAttribute('aria-setsize', '3000');
  // Only the rows near the viewport are in the page
  expect(await list.getByRole('listitem').count()).toBeLessThan(200);

  await list.evaluate((element) => {
    let node: HTMLElement | null = element;
    while (node && getComputedStyle(node).overflowY !== 'auto') {
      node = node.parentElement;
    }
    node?.scrollTo(0, node.scrollHeight);
  });
  await expect(list.getByRole('button', { name: /step 3000\b/ })).toBeVisible();
  await list.getByRole('button', { name: /step 3000\b/ }).click();
  await expect(page.getByRole('region', { name: 'Message' })).toContainText('row');
});

test('the views of a request: timeline, SQL, superglobals', async ({ page }) => {
  await openTheItemPage(page);
  const tabs = page.getByRole('tablist', { name: 'Request views' });

  await tabs.getByRole('tab', { name: /SQL/ }).click();
  const sql = page.getByRole('tabpanel', { name: /SQL/ });
  await expect(sql).toContainText('1 query');
  await expect(sql).toContainText('SELECT * FROM items WHERE id = ?');

  await tabs.getByRole('tab', { name: 'Timeline' }).click();
  const timeline = page.getByRole('tabpanel', { name: 'Timeline' });
  await expect(timeline.getByRole('button', { name: /Loading the item/ })).toBeVisible();
  await timeline.getByRole('button', { name: /db/ }).click();
  await expect(page.getByRole('region', { name: 'Message' })).toContainText('db');

  await tabs.getByRole('tab', { name: 'Superglobals' }).click();
  const globals = page.getByRole('tabpanel', { name: 'Superglobals' });
  await expect(globals).toContainText('$_GET');
  await expect(globals).toContainText('id');
});

test('the code around a message, read from the project', async ({ page }) => {
  await openTheItemPage(page);
  await page.getByRole('button', { name: /Loading the item/ }).click();
  const message = page.getByRole('region', { name: 'Message' });
  await message.getByRole('button', { name: 'Show the code' }).click();
  const code = message.getByRole('region', { name: 'public/index.php:13' });
  await expect(code).toContainText("__d(DWS_INFO, 'Loading the item'");
  await expect(code.locator('[data-current]')).toHaveText('13');
});

test('the copy buttons of the context are reachable from the keyboard', async ({ page }) => {
  await openTheItemPage(page);
  await page.getByRole('button', { name: /Loading the item/ }).click();
  const copy = page.getByRole('button', { name: 'Copy the path of item_id' });
  await copy.focus();
  await expect(copy).toBeFocused();
  await expect(copy).toBeVisible();
});
