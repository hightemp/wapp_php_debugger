<?php

declare(strict_types=1);

// A real fatal error: memory exhaustion. Argument: the directory with dws_debugger.json.
require dirname(__DIR__, 2) . '/vendor/autoload.php';

WappDebugger\Debugger::boot($argv[1] . '/dws_debugger.json');
ini_set('memory_limit', '16M');
$chunks = [];
while (true) {
    $chunks[] = str_repeat('x', 1024 * 1024);
}
