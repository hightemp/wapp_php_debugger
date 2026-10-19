<?php

declare(strict_types=1);

// The "application" for the HTTP mode test, served by php -S. WAPP_TEST_DIR holds dws_debugger.json.
require dirname(__DIR__, 3) . '/vendor/autoload.php';

WappDebugger\Debugger::boot(getenv('WAPP_TEST_DIR') . '/dws_debugger.json');
http_response_code(404);
echo 'the application answered';
