import { expect, test, type Page } from '@playwright/test';
import { resetSettings } from './settings';

// Appearance settings are kept by the viewer: every window and browser looks the same

test.beforeEach(({ request }) => resetSettings(request));

const root = (page: Page) =>
  page.evaluate(() => ({
    theme: document.documentElement.dataset.theme,
    density: document.documentElement.dataset.density,
    accent: document.documentElement.style.getPropertyValue('--accent'),
    bg: getComputedStyle(document.body).backgroundColor,
  }));

async function rowHeight(page: Page) {
  const row = page.getByRole('region', { name: 'Requests' }).getByRole('listitem').first();
  return (await row.boundingBox())?.height ?? 0;
}

test('appearance follows from one window to another and stays after a reload', async ({
  page,
  browser,
}) => {
  await page.goto('/');
  const comfortable = await rowHeight(page);
  await page.getByRole('button', { name: 'Appearance' }).click();
  const dialog = page.getByRole('dialog', { name: 'Appearance' });
  await dialog.getByRole('radio', { name: 'Compact' }).click();
  await dialog.getByRole('textbox', { name: 'Color of accent' }).fill('#d14d00');
  await expect.poll(() => root(page)).toMatchObject({ density: 'compact', accent: '#d14d00' });
  expect(await rowHeight(page)).toBeLessThan(comfortable);

  // Another browser: the settings of the viewer, not of this one
  const other = await (await browser.newContext()).newPage();
  await other.goto('/');
  await expect.poll(() => root(other)).toMatchObject({ density: 'compact', accent: '#d14d00' });

  // A change here reaches the open window at once
  await dialog.getByRole('radio', { name: 'Comfortable' }).click();
  await expect.poll(() => root(other)).toMatchObject({ density: 'comfortable' });

  await page.reload();
  expect((await root(page)).accent).toBe('#d14d00');
  await other.context().close();
});

test('a long list measures its rows again when the density changes', async ({ page, request }) => {
  await page.goto('/');
  await page
    .getByRole('region', { name: 'Requests' })
    .getByRole('button', { name: /3000 messages/ })
    .click();
  const list = page.getByRole('list', { name: 'Messages' });
  await expect(list.getByRole('listitem').first()).toBeVisible();
  // The list is as tall as its rows: 3000 of them, of the height they have now
  const fits = () =>
    list.evaluate((element) => {
      const row = element.querySelector('li') as HTMLElement;
      return (element.parentElement as HTMLElement).offsetHeight === 3000 * row.offsetHeight;
    });
  await expect.poll(fits).toBe(true);
  const before = await list.evaluate(
    (element) => (element.parentElement as HTMLElement).offsetHeight,
  );

  // From another window: the open list gets settings_updated
  await resetSettings(request, { density: 'compact' });
  await expect
    .poll(() => page.evaluate(() => document.documentElement.dataset.density))
    .toBe('compact');
  await expect.poll(fits).toBe(true);
  expect(
    await list.evaluate((element) => (element.parentElement as HTMLElement).offsetHeight),
  ).toBeLessThan(before);

  await list.evaluate((element) => {
    let node: HTMLElement | null = element;
    while (node && getComputedStyle(node).overflowY !== 'auto') {
      node = node.parentElement;
    }
    node?.scrollTo(0, node.scrollHeight);
  });
  await expect(list.getByRole('button', { name: /step 3000\b/ })).toBeVisible();
});

test('a custom theme is made in the editor and used', async ({ page }) => {
  await page.goto('/');
  const before = (await root(page)).bg;
  await page.getByRole('button', { name: 'Appearance' }).click();
  await page.getByRole('button', { name: 'Theme editor…' }).click();
  const editor = page.getByRole('dialog', { name: 'Theme editor' });
  await editor.getByRole('textbox', { name: 'Name' }).fill('Sepia');
  await editor.getByRole('textbox', { name: 'Color of bg', exact: true }).fill('#f4ecd8');
  await expect(editor.getByRole('region', { name: 'Preview' })).toHaveCSS(
    'background-color',
    'rgb(244, 236, 216)',
  );
  await editor.getByRole('textbox', { name: 'Color of text', exact: true }).fill('#dddddd');
  await expect(editor.getByRole('status')).toContainText('text on bg');
  await editor.getByRole('button', { name: /Back to the base color of text$/ }).click();
  await expect(editor.getByRole('status')).toContainText('Every pair of colors is readable');
  await editor.getByRole('button', { name: 'Save and use' }).click();

  await expect.poll(async () => (await root(page)).bg).toBe('rgb(244, 236, 216)');
  expect(before).not.toBe('rgb(244, 236, 216)');
  await page.reload();
  // The first frame already has it: theme-init.js reads the cache
  expect((await root(page)).bg).toBe('rgb(244, 236, 216)');

  const downloading = page.waitForEvent('download');
  await page.getByRole('button', { name: 'Appearance' }).click();
  await page.getByRole('button', { name: 'Theme editor…' }).click();
  await page.getByRole('button', { name: 'Export' }).click();
  const download = await downloading;
  expect(download.suggestedFilename()).toBe('sepia.theme.json');
});

test('the system theme follows the contrast the OS asks for', async ({ page }) => {
  await page.emulateMedia({ colorScheme: 'light', contrast: 'more' });
  await page.goto('/');
  expect((await root(page)).theme).toBe('high-contrast');
  await page.emulateMedia({ colorScheme: 'dark', contrast: 'no-preference' });
  await expect.poll(async () => (await root(page)).theme).toBe('dark');
});

test('the component showcase opens from the palette', async ({ page }) => {
  const errors: string[] = [];
  page.on('pageerror', (error) => errors.push(error.message));
  page.on('console', (message) => message.type() === 'error' && errors.push(message.text()));
  await page.goto('/');
  await page.keyboard.press('Control+k');
  await page.getByRole('combobox', { name: 'Command' }).fill('showcase');
  await page.keyboard.press('Enter');
  await expect(page).toHaveURL(/#\/dev\/components$/);
  await expect(page.getByRole('region', { name: /^Theme / })).toHaveCount(3);
  expect(errors).toEqual([]);
});
