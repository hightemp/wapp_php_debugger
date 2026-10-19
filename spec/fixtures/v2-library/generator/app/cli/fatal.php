<?php

// Runs out of memory: a fatal error caught at shutdown.
require '/var/www/lib/autoload.php';

WappDebugger\Debugger::boot('/var/www/app/dws_debugger.json');

ini_set('memory_limit', '16M');
$chunks = [];
while (true) {
    $chunks[] = str_repeat('x', 1024 * 1024);
}
