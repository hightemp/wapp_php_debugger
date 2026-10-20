import { createServer } from 'node:http';
import type { AddressInfo } from 'node:net';
import { fileURLToPath } from 'node:url';
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

test('a request exported as a file comes back by import', async ({ page }) => {
  await openTheItemPage(page);
  const downloading = page.waitForEvent('download');
  await page.getByRole('link', { name: 'Export' }).click();
  const download = await downloading;
  expect(download.suggestedFilename()).toMatch(/^request-.+\.jsonl$/);
  const path = await download.path();

  await page
    .getByRole('navigation', { name: 'Projects' })
    .getByRole('button', { name: 'shop' })
    .click();
  await page.getByLabel('Import a request').setInputFiles(path);
  await expect(page.getByText('The project has this request already')).toBeVisible();
  await expect(
    page.getByRole('heading', { level: 2, name: /\/catalog\/item\?id=42/ }),
  ).toBeVisible();
});

test('two open requests side by side', async ({ page }) => {
  await openTheItemPage(page);
  const feed = page.getByRole('region', { name: 'Requests' });
  await feed.getByRole('button', { name: /POST \/api\/cart\/add/ }).click();
  await page.getByRole('button', { name: /^Compare with GET \/catalog\/item/ }).click();
  const comparison = page.getByRole('region', { name: 'Comparison' });
  await expect(comparison.getByRole('table', { name: 'The requests' })).toBeVisible();
  await expect(comparison.getByRole('row', { name: /^URL/ })).toContainText('/api/cart/add');
  await page.screenshot({ path: test.info().outputPath('comparison.png') });
  await page.keyboard.press('Escape');
  await expect(comparison).toBeHidden();
});

test('errors of a page come in as a child of its PHP request', async ({ page, request }) => {
  const made = await request.post('/api/v1/projects/1/ingest-token');
  const { token } = (await made.json()) as { token: string };
  const feed = (await (await request.get('/api/v1/requests?limit=100')).json()) as {
    items: { request_key: string; url?: string | null }[];
  };
  const parent = feed.items.find((item) => item.url?.includes('/catalog/item'));
  expect(parent).toBeDefined();
  const ingest = `http://127.0.0.1:${Number(process.env.E2E_PORT ?? 18840) + 1}`;

  // The shop page as PHP would print it: Debugger::jsTag(), then a script that fails. A real local
  // server: Chrome lets a page reach a local receiver only from the same address space.
  const shop = createServer((_, response) => {
    response.setHeader('Content-Type', 'text/html');
    response.end(`<!doctype html><title>Item</title>
      <script src="${ingest}/ingest/debugger.js" data-token="${token}" data-request-id="${parent?.request_key}"></script>
      <script>setTimeout(function () { var item; item.price; }, 50);</script>`);
  });
  await new Promise<void>((done) => shop.listen(0, '127.0.0.1', done));
  const { port } = shop.address() as AddressInfo;
  try {
    await page.goto(`http://127.0.0.1:${port}/catalog/item?id=42`);
    await page.waitForTimeout(300);
    await page.goto('/'); // leaving the page sends the rest with a beacon
  } finally {
    shop.close();
  }

  const requests = page.getByRole('region', { name: 'Requests' });
  const child = requests.getByRole('button', { name: /GET \/catalog\/item\?id=42.*1 error/ });
  await expect(child).toBeVisible({ timeout: 10_000 });
  await child.click();
  await expect(
    page.getByRole('button', { name: /Cannot read properties of undefined/ }),
  ).toBeVisible();
});

test('a Laravel log comes in as a request of its own', async ({ page }) => {
  await page.goto('/');
  await page
    .getByRole('navigation', { name: 'Projects' })
    .getByRole('button', { name: 'shop' })
    .click();
  await page
    .getByLabel('Import a request')
    .setInputFiles(
      fileURLToPath(new URL('../../spec/fixtures/thirdparty/laravel.log', import.meta.url)),
    );
  await expect(page.getByRole('heading', { level: 2, name: 'CLI' })).toBeVisible();
  await expect(page.getByText('laravel.log', { exact: true })).toBeVisible();
  const messages = page.getByRole('list', { name: 'Messages' });
  await expect(messages).toContainText('User logged in');
  await expect(messages).toContainText('SQLSTATE[42S02]: Base table or view not found');
  await page.screenshot({ path: test.info().outputPath('laravel.png') });
});
