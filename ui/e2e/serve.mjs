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

// A long CLI run: thousands of messages in one request (virtual lists, paging of messages)
const bigId = '01M5AKMCB0000000000000BIG1';
const bigStart = 1792431240000;
const lines = [
    {
        v: 2,
        type: 'request_start',
        request_id: bigId,
        seq: 1,
        ts: bigStart,
        lang: 'php',
        library_version: '2.0.0',
        php_version: '7.4.33',
        sapi: 'cli',
        argv: ['/var/www/app/cli/import.php'],
        hostname: 'fixture',
        pid: 99,
    },
];
for (let i = 1; i <= 3000; i++) {
    lines.push({
        v: 2,
        type: 'message',
        request_id: bigId,
        seq: i + 1,
        ts: bigStart + i,
        level: 'info',
        message: `step ${i}`,
        tags: ['import'],
        context: { row: i },
    });
}
lines.push({
    v: 2,
    type: 'request_end',
    request_id: bigId,
    seq: 3002,
    ts: bigStart + 3001,
    duration_ms: 3001,
    memory_peak_bytes: 2097152,
    exit_code: 0,
});
writeFileSync(
    join(logs, `${bigStart}-${bigId}.jsonl`),
    lines.map((line) => JSON.stringify(line)).join('\n') + '\n',
);

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
