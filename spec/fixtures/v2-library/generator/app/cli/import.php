<?php

// A CLI job that fails with an uncaught exception that has a previous one.
require '/var/www/lib/autoload.php';

WappDebugger\Debugger::boot('/var/www/app/dws_debugger.json');

function importOrders(string $file): void
{
    try {
        throw new InvalidArgumentException("Cannot open $file", 2);
    } catch (InvalidArgumentException $exception) {
        throw new RuntimeException('Import failed', 0, $exception);
    }
}

__d(DWS_INFO, 'Import started', ['argv' => $argv]);
importOrders('/data/orders.csv');
