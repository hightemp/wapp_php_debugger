// Starts the built viewer (dist/wapp-debugger) on a copy of the v2 fixtures written by the real
// library, for Playwright: one project, its logs named the way the library names them.
import { spawn } from 'node:child_process';
import {
    copyFileSync,
    mkdirSync,
    mkdtempSync,
    readdirSync,
    readFileSync,
    rmSync,
    writeFileSync,
} from 'node:fs';
import { tmpdir } from 'node:os';
import { dirname, join, resolve } from 'node:path';
import { fileURLToPath } from 'node:url';

const repo = resolve(dirname(fileURLToPath(import.meta.url)), '../..');
const fixtures = join(repo, 'spec/fixtures/v2-library');
const port = process.env.E2E_PORT ?? '18840';

const work = mkdtempSync(join(tmpdir(), 'wapp-debugger-e2e-'));
const root = join(work, 'shop');
const logs = join(root, 'tmp', 'logs');
mkdirSync(logs, { recursive: true });
writeFileSync(join(root, 'dws_debugger.json'), '{"enabled": true, "logs_dir": "tmp/logs"}\n');
for (const name of readdirSync(fixtures).filter((file) => file.endsWith('.jsonl'))) {
    const first = JSON.parse(readFileSync(join(fixtures, name), 'utf8').split('\n')[0]);
    copyFileSync(join(fixtures, name), join(logs, `${first.ts}-${first.request_id}.jsonl`));
}

const viewer = spawn(
    join(repo, 'dist', process.platform === 'win32' ? 'wapp-debugger.exe' : 'wapp-debugger'),
    ['serve', '--addr', `127.0.0.1:${port}`, '--db', join(work, 'viewer.db'), '--logs-dir', logs],
    { stdio: 'inherit' },
);
const stop = () => viewer.kill('SIGTERM');
process.on('SIGTERM', stop);
process.on('SIGINT', stop);
viewer.on('exit', (code) => {
    rmSync(work, { recursive: true, force: true });
    process.exit(code ?? 0);
});
