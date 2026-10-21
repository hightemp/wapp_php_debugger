<?php

/*
 * wapp-debugger for 1C-Bitrix: require this file at the top of /local/php_interface/init.php
 * (or /bitrix/php_interface/init.php):
 *     require __DIR__ . '/wapp-debugger.php';
 *
 * The library comes from Composer (vendor/autoload.php required before) or from the one-file build
 * of a release next to this file (wapp-debugger-library.php). The config is
 * WAPP_DEBUGGER_CONFIG, else DOCUMENT_ROOT/dws_debugger.json. Off without it.
 */

declare(strict_types=1);

(static function (): void {
    if (!class_exists(\WappDebugger\Debugger::class) && is_file(__DIR__ . '/wapp-debugger-library.php')) {
        require_once __DIR__ . '/wapp-debugger-library.php';
    }
    if (!class_exists(\WappDebugger\Debugger::class)) {
        return; // no library: the site works as before
    }

    $root = rtrim((string) ($_SERVER['DOCUMENT_ROOT'] ?? ''), '/');
    $debugger = \WappDebugger\Debugger::boot(defined('WAPP_DEBUGGER_CONFIG') ? (string) constant('WAPP_DEBUGGER_CONFIG') : $root . '/dws_debugger.json');
    if (!$debugger->enabled()) {
        return;
    }

    // Errors of the pages come in as a request of their own, a child of this one
    AddEventHandler('main', 'OnEndBufferContent', static function (&$content) use ($debugger): void {
        $tag = $debugger->jsTag();
        $at = is_string($content) ? strripos($content, '</body>') : false;
        if ($tag !== '' && $at !== false) {
            $content = substr_replace($content, $tag, $at, 0);
        }
    });

    // Queries of the D7 connection (the old $DB goes through it too), with their time
    if (class_exists(\Bitrix\Main\Application::class)) {
        $tracker = \Bitrix\Main\Application::getConnection()->startTracker();
        AddEventHandler('main', 'OnAfterEpilog', static function () use ($debugger, $tracker): void {
            foreach ($tracker->getQueries() as $query) {
                $debugger->sql((string) $query->getSql(), [], (float) $query->getTime() * 1000);
            }
        });
    }
})();
