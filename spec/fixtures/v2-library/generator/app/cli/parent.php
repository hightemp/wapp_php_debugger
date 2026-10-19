<?php

// A CLI job that starts a child process: the child links itself to this request.
require '/var/www/lib/autoload.php';

WappDebugger\Debugger::boot('/var/www/app/dws_debugger.json');

__d(DWS_MESSAGE, 'Starting the child');
passthru(PHP_BINARY . ' ' . escapeshellarg(__DIR__ . '/child.php'));
__d(DWS_MESSAGE, 'Child finished');
