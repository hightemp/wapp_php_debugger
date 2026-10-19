<?php

declare(strict_types=1);

// make php-bundle-test loads the bundle with auto_prepend_file, before PHPUnit loads Composer's
// autoloader; here we only make sure the classes really come from the bundle.
require dirname(__DIR__) . '/vendor/autoload.php';

$bundle = realpath(dirname(__DIR__, 2) . '/dist/wapp-debugger.php');
if ($bundle === false || (new ReflectionClass(WappDebugger\Debugger::class))->getFileName() !== $bundle) {
    fwrite(STDERR, "the library is not loaded from dist/wapp-debugger.php: run make php-bundle-test\n");
    exit(1);
}
