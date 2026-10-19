<?php

declare(strict_types=1);

// A real process: a warning, then an uncaught exception. Argument: the directory with dws_debugger.json.
require dirname(__DIR__, 2) . '/vendor/autoload.php';

$debugger = WappDebugger\Debugger::boot($argv[1] . '/dws_debugger.json');
$debugger->log('info', 'started');
$items = [];
echo $items['missing'] ?? '', @$items['silenced'];
trigger_error('Undefined index: discount', E_USER_WARNING);

function load(int $id): void
{
    throw new RuntimeException("Order $id not found", 404);
}
load(7);
