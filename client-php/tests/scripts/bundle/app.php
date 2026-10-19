<?php

declare(strict_types=1);

// A project without Composer: one require of the bundle next to this file. Argument: the project root.
require __DIR__ . '/wapp-debugger.php';

WappDebugger\Debugger::boot($argv[1] . '/dws_debugger.json');

__d(DWS_INFO, 'Loaded', ['id' => 7, 'password' => 'secret']);
__start('work');
__stop('work');

throw new LogicException('stop here');
