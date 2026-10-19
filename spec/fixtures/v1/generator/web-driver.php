<?php
// Выполняет веб-сценарии через запущенный php -S и складывает лог каждого запроса в <out>/<name>.log
// Запуск: php web-driver.php <logs-dir> <out-dir>

[, $logsDir, $outDir] = $argv;

$scenarios = [
    'web-get' => ['GET', '/scenario.php?id=42', [], ''],
    'web-ajax-post' => ['POST', '/scenario.php', ['X-Requested-With: XMLHttpRequest', 'Content-Type: application/x-www-form-urlencoded'], 'item_id=42&qty=2&password=should-be-masked'],
    'long-url' => ['GET', '/scenario.php?q='.rawurlencode(str_repeat('очень-длинный-запрос-', 120)), [], ''],
];

foreach ($scenarios as $name => [$method, $uri, $headers, $body]) {
    $context = stream_context_create(['http' => [
        'method' => $method,
        'header' => array_merge(['Host: shop.example.test', 'User-Agent: fixture-agent', "X-Fixture-Scenario: {$name}"], $headers),
        'content' => $body,
        'ignore_errors' => true,
    ]]);
    $response = file_get_contents('http://127.0.0.1:8080'.$uri, false, $context);
    usleep(300000); // shutdown-функция дописывает лог после ответа

    $files = glob($logsDir.'/*');
    if (count($files) !== 1) {
        fwrite(STDERR, "{$name}: expected 1 log file, got ".count($files).", response: {$response}\n");
        exit(1);
    }
    rename($files[0], "{$outDir}/{$name}.log");
}
