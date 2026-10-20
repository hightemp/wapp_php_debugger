// Renders desktop/icons/icon.svg into the icons of the desktop app and its installers:
// PNGs (tray, Linux), icon.ico (Windows) and icon.icns (macOS). Chromium from Playwright does the
// drawing, the containers are written here. Run: make icons (needs ui deps and a Playwright Chromium).
import { readFileSync, writeFileSync } from 'node:fs';
import { dirname, join } from 'node:path';
import { fileURLToPath } from 'node:url';
import { chromium } from '../ui/node_modules/@playwright/test/index.mjs';

const dir = join(dirname(fileURLToPath(import.meta.url)), '..', 'desktop', 'icons');
const svg = readFileSync(join(dir, 'icon.svg'), 'utf8');
const sizes = [16, 22, 32, 44, 48, 64, 128, 256, 512, 1024];

const browser = await chromium.launch();
const page = await browser.newPage();
const png = {};
for (const size of sizes) {
  await page.setViewportSize({ width: size, height: size });
  await page.setContent(
    `<html><body style="margin:0;background:transparent">${svg.replace('<svg ', `<svg width="${size}" height="${size}" `)}</body></html>`,
  );
  png[size] = await page.screenshot({ omitBackground: true, clip: { x: 0, y: 0, width: size, height: size } });
}
await browser.close();

for (const size of [32, 44, 64, 256, 512, 1024]) {
  writeFileSync(join(dir, `icon-${size}.png`), png[size]);
}
// The tray icon is embedded into the desktop binary, so it lives next to its code
writeFileSync(join(dir, '..', '..', 'server', 'cmd', 'wapp-debugger-desktop', 'tray.png'), png[64]);

// ICO with PNG entries (Windows Vista and later)
const icoSizes = [16, 32, 48, 256];
const header = Buffer.alloc(6 + 16 * icoSizes.length);
header.writeUInt16LE(0, 0);
header.writeUInt16LE(1, 2);
header.writeUInt16LE(icoSizes.length, 4);
let offset = header.length;
icoSizes.forEach((size, i) => {
  const entry = 6 + 16 * i;
  header.writeUInt8(size >= 256 ? 0 : size, entry);
  header.writeUInt8(size >= 256 ? 0 : size, entry + 1);
  header.writeUInt16LE(1, entry + 4); // planes
  header.writeUInt16LE(32, entry + 6); // bits per pixel
  header.writeUInt32LE(png[size].length, entry + 8);
  header.writeUInt32LE(offset, entry + 12);
  offset += png[size].length;
});
writeFileSync(join(dir, 'icon.ico'), Buffer.concat([header, ...icoSizes.map((size) => png[size])]));

// ICNS with PNG entries: ic07 128, ic08 256, ic09 512, ic10 1024 (512@2x), ic13 256 (128@2x), ic14 512 (256@2x)
const icns = [
  ['ic07', 128],
  ['ic08', 256],
  ['ic09', 512],
  ['ic10', 1024],
  ['ic13', 256],
  ['ic14', 512],
].map(([type, size]) => {
  const chunk = Buffer.alloc(8);
  chunk.write(type, 0, 'ascii');
  chunk.writeUInt32BE(8 + png[size].length, 4);
  return Buffer.concat([chunk, png[size]]);
});
const total = 8 + icns.reduce((sum, chunk) => sum + chunk.length, 0);
const icnsHeader = Buffer.alloc(8);
icnsHeader.write('icns', 0, 'ascii');
icnsHeader.writeUInt32BE(total, 4);
writeFileSync(join(dir, 'icon.icns'), Buffer.concat([icnsHeader, ...icns]));
console.log(`icons written to ${dir}`);
