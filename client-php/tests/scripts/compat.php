<?php

declare(strict_types=1);

// A project as it was on library v1: no Composer, a v1 config, only the global helpers.
// Argument: the directory with dws_debugger.json.
require dirname(__DIR__, 2) . '/src/autoload.php';

WappDebugger\Debugger::boot($argv[1] . '/dws_debugger.json');

__start('total');
__d(DWS_INFO, 'Loading the order', ['id' => 7, 'password' => 'secret']);
$total = __log(40) + 2;
__stop('total');
