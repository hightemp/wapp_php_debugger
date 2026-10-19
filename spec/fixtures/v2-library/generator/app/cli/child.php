<?php

require '/var/www/lib/autoload.php';

WappDebugger\Debugger::boot('/var/www/app/dws_debugger.json');

__d(DWS_MESSAGE, 'Child at work', ['pid' => getmypid()]);
