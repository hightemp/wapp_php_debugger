<?php

// Runs inside the container: drives the scenarios, then names the logs and writes manifest.json.
// Usage: php driver.php <logs dir> <output dir>
[, $logs, $out] = $argv;

function http(string $method, string $path, array $headers = [], string $body = ''): string
{
    $context = stream_context_create(['http' => ['method' => $method, 'header' => $headers, 'content' => $body, 'ignore_errors' => true]]);

    return (string) file_get_contents('http://127.0.0.1:8080' . $path, false, $context);
}

function newLogs(string $logs, array &$seen): array
{
    $files = array_values(array_diff(glob($logs . '/*.jsonl'), $seen));
    sort($files);
    $seen = array_merge($seen, $files);

    return $files;
}

function cli(string $script, array $args = []): void
{
    $command = PHP_BINARY . ' /var/www/app/cli/' . $script;
    foreach ($args as $arg) {
        $command .= ' ' . escapeshellarg($arg);
    }
    exec($command . ' 2>/dev/null');
}

$seen = [];
$named = [];

$pageId = trim(http('GET', '/catalog/item?id=42'));
[$named['web-page.jsonl']] = newLogs($logs, $seen);
http('POST', '/api/cart/add', ['X-Wapp-Debugger-Parent: ' . $pageId, 'X-Requested-With: XMLHttpRequest', 'Content-Type: application/x-www-form-urlencoded'], 'sku=A-42&qty=2');
[$named['web-ajax.jsonl']] = newLogs($logs, $seen);

cli('import.php', ['--file=/data/orders.csv']);
[$named['cli-exception.jsonl']] = newLogs($logs, $seen);

cli('parent.php');
// Both files start within the same millisecond range; tell them apart by the script name
foreach (newLogs($logs, $seen) as $file) {
    $start = json_decode(strtok((string) file_get_contents($file), "\n"), true);
    $named[strpos($start['argv'][0], 'child.php') !== false ? 'cli-child.jsonl' : 'cli-parent.jsonl'] = $file;
}

cli('fatal.php');
[$named['cli-fatal.jsonl']] = newLogs($logs, $seen);

foreach (['web-page.jsonl', 'web-ajax.jsonl', 'cli-exception.jsonl', 'cli-parent.jsonl', 'cli-child.jsonl', 'cli-fatal.jsonl'] as $name) {
    if (!isset($named[$name]) || !is_string($named[$name])) {
        fwrite(STDERR, "scenario $name left no log\n");
        exit(1);
    }
}

$manifest = ['files' => []];
ksort($named);
foreach ($named as $name => $file) {
    $events = array_map(static function (string $line): array {
        return json_decode($line, true);
    }, explode("\n", rtrim((string) file_get_contents($file), "\n")));
    $manifest['files'][$name] = array_filter([
        'request_id' => $events[0]['request_id'],
        'parent_request_id' => $events[0]['parent_request_id'] ?? null,
        'types' => array_column($events, 'type'),
    ], static function ($value): bool {
        return $value !== null;
    });
    copy($file, $out . '/' . $name);
}
file_put_contents($out . '/manifest.json', json_encode($manifest, JSON_PRETTY_PRINT | JSON_UNESCAPED_SLASHES) . "\n");
