<?php

declare(strict_types=1);

namespace WappDebugger\Bridge;

use WappDebugger\Level;

/**
 * PSR-3 (and Monolog) levels onto the five levels of format v2.
 *
 * @internal
 */
final class Levels
{
    public const PSR3 = [
        'emergency' => Level::ERROR,
        'alert' => Level::ERROR,
        'critical' => Level::ERROR,
        'error' => Level::ERROR,
        'warning' => Level::WARNING,
        'notice' => Level::INFO,
        'info' => Level::INFO,
        'debug' => Level::DEBUG,
    ];

    private function __construct()
    {
    }

    /**
     * The directory of the class's file, to skip its frames when looking for the call site.
     *
     * @param class-string $class
     */
    public static function dirOf(string $class): string
    {
        $file = (new \ReflectionClass($class))->getFileName();

        return $file === false ? '' : dirname($file);
    }
}
