<?php

/*
 * Plugin Name: wapp-debugger
 * Description: Sends the events of every request to the wapp-debugger viewer: errors, queries (with SAVEQUERIES), errors of the pages. Does nothing without dws_debugger.json.
 * Version: 2.0.0
 * Requires PHP: 7.4
 * License: MIT
 *
 * A must-use plugin: copy this file to wp-content/mu-plugins/. The library is found by the
 * WAPP_DEBUGGER_LIBRARY constant (wp-config.php), else next to this file (wapp-debugger-library.php,
 * the one-file build of a release), else in vendor/ of the site or of its parent (Composer, Bedrock).
 * The config is WAPP_DEBUGGER_CONFIG, else ABSPATH/dws_debugger.json.
 */

declare(strict_types=1);

if (!defined('ABSPATH')) {
    return;
}

(static function (): void {
    if (!class_exists(\WappDebugger\Debugger::class)) {
        $candidates = [
            defined('WAPP_DEBUGGER_LIBRARY') ? (string) constant('WAPP_DEBUGGER_LIBRARY') : '',
            __DIR__ . '/wapp-debugger-library.php',
            ABSPATH . 'vendor/autoload.php',
            dirname(ABSPATH) . '/vendor/autoload.php',
        ];
        foreach ($candidates as $file) {
            if ($file !== '' && is_file($file)) {
                require_once $file;

                break;
            }
        }
        if (!class_exists(\WappDebugger\Debugger::class)) {
            return; // no library: the site works as before
        }
    }

    $debugger = \WappDebugger\Debugger::boot(defined('WAPP_DEBUGGER_CONFIG') ? (string) constant('WAPP_DEBUGGER_CONFIG') : ABSPATH . 'dws_debugger.json');
    if (!$debugger->enabled()) {
        return;
    }

    // Errors of the pages come in as a request of their own, a child of this one
    $tag = static function () use ($debugger): void {
        echo $debugger->jsTag();
    };
    add_action('wp_footer', $tag, 1000);
    add_action('admin_footer', $tag, 1000);
    add_action('login_footer', $tag, 1000);

    // With SAVEQUERIES on, $wpdb passes each query with its time (seconds) right after it ran
    if (defined('SAVEQUERIES') && constant('SAVEQUERIES')) {
        add_filter('log_query_custom_data', static function ($data, $query, $seconds) use ($debugger) {
            $debugger->sql((string) $query, [], is_numeric($seconds) ? (float) $seconds * 1000 : null);

            return $data;
        }, 10, 3);
    }
})();
