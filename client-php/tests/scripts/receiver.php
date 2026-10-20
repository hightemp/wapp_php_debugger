<?php

declare(strict_types=1);

// A receiver of pushed logs for tests (php -S router): keeps what came, answers like the viewer.
$dir = (string) getenv('WAPP_TEST_DIR');
file_put_contents($dir . '/pushed.jsonl', (string) file_get_contents('php://input'), FILE_APPEND);
file_put_contents($dir . '/headers.txt', ($_SERVER['HTTP_AUTHORIZATION'] ?? '') . ' ' . ($_SERVER['CONTENT_TYPE'] ?? '') . "\n", FILE_APPEND);
header('Content-Type: application/json');
echo '{"created":1,"updated":0,"rejected":0}';
