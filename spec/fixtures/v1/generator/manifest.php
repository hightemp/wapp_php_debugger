<?php
// Описывает сгенерированные фикстуры: эталон для тестов парсера v1 (Go, фаза 4).
// Запуск: php manifest.php <dir> > manifest.json

$descriptions = [
    'web-get' => 'Обычный GET-запрос: все типы сообщений, таймер, __log, E_USER_ERROR, поля найдено/не найдено',
    'web-ajax-post' => 'AJAX POST с паролем в $_POST (замаскирован hotfix-версией)',
    'cli' => 'CLI-запуск с аргументами (bIsCli=true, aArgv заполнен)',
    'long-url' => 'URL длиннее 1000 байт: hotfix-версия укорачивает sURL до 300 байт, заголовок ровно 1000 байт',
    'killed' => 'Процесс убит SIGKILL до shutdown: bFinished=false, нет итоговых сообщений',
    'truncated-last-line' => 'Копия web-get, последняя строка оборвана посередине (сбой во время записи)',
    'pre-hotfix-long-header' => 'Библиотека до hotfix: заголовок длиннее 1000 байт затёр начало первого сообщения',
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
