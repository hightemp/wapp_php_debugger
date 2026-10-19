<?php

declare(strict_types=1);

/*
 * For projects without Composer: require this file, then call WappDebugger\Debugger::boot().
 * With Composer the package autoloads itself (and loads functions.php).
 */
spl_autoload_register(static function (string $class): void {
    if (strpos($class, 'WappDebugger\\') === 0) {
        $file = __DIR__ . '/' . str_replace('\\', '/', substr($class, strlen('WappDebugger\\'))) . '.php';
        if (is_file($file)) {
            require $file;
        }
    }
});

require_once __DIR__ . '/functions.php';
