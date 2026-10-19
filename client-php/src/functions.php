<?php

declare(strict_types=1);

/*
 * Global helpers of library v1. __d, __log, __start and __stop stay for good (plan 10.23);
 * the fn* functions and the DWS_* constants are deprecated and go away in 3.0.
 * They write to Debugger::instance(); without Debugger::boot() they do nothing.
 */

use WappDebugger\Compat;
use WappDebugger\Debugger;
use WappDebugger\Level;

defined('DWS_ERROR') || define('DWS_ERROR', Level::ERROR);
defined('DWS_WARNING') || define('DWS_WARNING', Level::WARNING);
defined('DWS_INFO') || define('DWS_INFO', Level::INFO);
defined('DWS_MESSAGE') || define('DWS_MESSAGE', Level::MESSAGE);
defined('DWS_FUNC_DEBUG') || define('DWS_FUNC_DEBUG', 'functions_debug');

if (!function_exists('__d')) {
    /**
     * Writes a message: __d(DWS_ERROR, 'Order failed', ['order' => $order]).
     *
     * @param mixed $message
     * @param mixed $context named variables; anything else is written as {"value": …}
     * @param int   $shift   ignored, kept for library v1 calls: the call site is found by itself
     */
    function __d(string $level, $message, $context = [], int $shift = 1): void
    {
        $debugger = Debugger::instance();
        if ($debugger->enabled()) {
            $debugger->log($level, Compat::text($message), is_array($context) ? $context : ['value' => $context]);
        }
    }
}

if (!function_exists('__log')) {
    /**
     * Writes a value and returns it, to log right inside an expression: f(__log($a)).
     *
     * @param mixed $value
     *
     * @return mixed the value
     */
    function __log($value = [])
    {
        $debugger = Debugger::instance();
        if ($debugger->enabled()) {
            $first = is_array($value) && count($value) === 1 ? reset($value) : null;
            $text = is_string($value) || is_numeric($value) ? (string) $value : (is_scalar($first) ? (string) $first : '');
            $type = gettype($value);
            $debugger->log(Level::MESSAGE, $text === '' ? $type : $text, is_array($value) ? $value : ['value' => $value], ['inlinelog', $type]);
        }

        return $value;
    }
}

if (!function_exists('__start')) {
    /** Starts a timer; unnamed timers form a stack. */
    function __start(string $name = ''): void
    {
        Debugger::instance()->startTimer($name);
    }
}

if (!function_exists('__stop')) {
    /**
     * Stops a timer and writes its duration.
     *
     * @param int $shift ignored, kept for library v1 calls
     */
    function __stop(string $name = '', int $shift = 2): void
    {
        Debugger::instance()->stopTimer($name);
    }
}

if (!function_exists('fnDebugLog')) {
    /**
     * @deprecated since 2.0, use __d()
     *
     * @param mixed $message
     * @param mixed $context
     */
    function fnDebugLog(string $level, $message, $context = []): void // naming-check:ignore (library v1 name)
    {
        Compat::deprecated('fnDebugLog', '__d');
        __d($level, $message, $context);
    }
}

if (!function_exists('fnDebugAddFoundField')) {
    /**
     * @deprecated since 2.0, use __d() with a tag
     */
    function fnDebugAddFoundField(string $field): void // naming-check:ignore (library v1 name)
    {
        Compat::deprecated('fnDebugAddFoundField', '__d');
        Debugger::instance()->log(Level::DEBUG, $field, [], ['found_field']);
    }
}

if (!function_exists('fnDebugAddNotFoundField')) {
    /**
     * @deprecated since 2.0, use __d() with a tag
     */
    function fnDebugAddNotFoundField(string $field): void // naming-check:ignore (library v1 name)
    {
        Compat::deprecated('fnDebugAddNotFoundField', '__d');
        Debugger::instance()->log(Level::DEBUG, $field, [], ['not_found_field']);
    }
}
