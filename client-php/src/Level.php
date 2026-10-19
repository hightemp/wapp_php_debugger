<?php

declare(strict_types=1);

namespace WappDebugger;

/**
 * Message levels of format v2. A class with constants: enums need PHP 8.1, the library supports 7.4.
 */
final class Level
{
    public const ERROR = 'error';
    public const WARNING = 'warning';
    public const INFO = 'info';
    /** A neutral entry, like DWS_MESSAGE in v1. */
    public const MESSAGE = 'message';
    public const DEBUG = 'debug';

    private const ALL = [self::ERROR, self::WARNING, self::INFO, self::MESSAGE, self::DEBUG];

    private function __construct()
    {
    }

    /** Unknown levels become MESSAGE: logging must never break the application. */
    public static function normalize(string $level): string
    {
        $level = strtolower($level);

        return in_array($level, self::ALL, true) ? $level : self::MESSAGE;
    }
}
