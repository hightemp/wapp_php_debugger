<?php
// Describes the generated fixtures: the reference for v1 parser tests.
// Usage: php manifest.php <dir> > manifest.json

$descriptions = [
    'web-get' => 'Plain GET request: all message types, a timer, __log, E_USER_ERROR, found/not found fields',
    'web-ajax-post' => 'AJAX POST with a password in $_POST (masked by the hotfix version)',
    'cli' => 'CLI run with arguments (bIsCli=true, aArgv filled)',
    'long-url' => 'URL longer than 1000 bytes: the hotfix version shortens sURL to 300 bytes, the header is exactly 1000 bytes',
    'killed' => 'Process killed with SIGKILL before shutdown: bFinished=false, no final messages',
    'truncated-last-line' => 'Copy of web-get with the last line cut in the middle (crash during a write)',
    'pre-hotfix-long-header' => 'Library before the hotfix: a header longer than 1000 bytes overwrote the start of the first message',
];

$fixtures = [];
foreach (glob($argv[1].'/*.log') as $path) {
    $name = basename($path, '.log');
    $content = file_get_contents($path);
    $lines = explode("\n", $content);
    $headerLine = array_shift($lines);
    $header = json_decode($headerLine, true);

    $valid = 0;
    $broken = 0;
    $levels = [];
    foreach ($lines as $line) {
        if ($line === '') {
            continue;
        }
        $message = json_decode($line, true);
        if (is_array($message) && isset($message['sType'])) {
            $valid++;
            $levels[$message['sType']] = ($levels[$message['sType']] ?? 0) + 1;
        } else {
            $broken++;
        }
    }
    ksort($levels);

    $fixtures[] = [
        'file' => $name.'.log',
        'description' => $descriptions[$name] ?? '',
        'size' => strlen($content),
        'header_valid' => is_array($header),
        'header_length' => strlen($headerLine),
        'finished' => is_array($header) ? $header['bFinished'] : null,
        'is_cli' => is_array($header) ? $header['bIsCli'] : null,
        'is_ajax' => is_array($header) ? $header['bIsAjax'] : null,
        'url' => is_array($header) ? $header['sURL'] : null,
        'messages_valid' => $valid,
        'messages_broken' => $broken,
        'levels' => $levels,
        'ends_with_newline' => substr($content, -1) === "\n",
    ];
}

echo json_encode(['format' => 'v1', 'fixtures' => $fixtures], JSON_PRETTY_PRINT | JSON_UNESCAPED_UNICODE | JSON_UNESCAPED_SLASHES), "\n";
