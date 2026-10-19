<?php

declare(strict_types=1);

namespace WappDebugger;

/**
 * Support for the library v1 helpers in functions.php.
 *
 * @internal
 */
final class Compat
{
    /** @var array<string, true> */
    private static array $warned = [];

    private function __construct()
    {
    }

    /** One E_USER_DEPRECATED per function and request: enough to notice, not enough to flood the logs. */
    public static function deprecated(string $function, string $replacement): void
    {
        if (isset(self::$warned[$function])) {
            return;
        }
        self::$warned[$function] = true;
        @trigger_error(sprintf('%s() is deprecated, use %s() instead; it goes away in wapp-debugger 3.0', $function, $replacement), E_USER_DEPRECATED);
    }

    /**
     * @param mixed $message v1 accepted any scalar
     */
    public static function text($message): string
    {
        if ($message === null) {
            return '';
        }
        if (is_bool($message)) {
            return $message ? 'true' : 'false';
        }

        return is_scalar($message) ? (string) $message : gettype($message);
    }
}
