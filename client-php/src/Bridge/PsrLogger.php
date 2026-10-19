<?php

declare(strict_types=1);

namespace WappDebugger\Bridge;

use Psr\Log\AbstractLogger;
use Psr\Log\InvalidArgumentException;
use WappDebugger\Debugger;
use WappDebugger\Frames;

/**
 * A PSR-3 logger writing to the viewer. One class for psr/log 1, 2 and 3: the untyped $message
 * and the void return are compatible with all three interfaces.
 */
final class PsrLogger extends AbstractLogger
{
    private Debugger $debugger;

    public function __construct(?Debugger $debugger = null)
    {
        $this->debugger = $debugger ?? Debugger::instance();
        Frames::skipDirectory(Levels::dirOf(AbstractLogger::class));
    }

    /**
     * @param mixed                $level
     * @param string|\Stringable   $message
     * @param array<string, mixed> $context
     */
    public function log($level, $message, array $context = []): void
    {
        if (!is_string($level) || !isset(Levels::PSR3[$level])) {
            throw new InvalidArgumentException(sprintf('Unknown PSR-3 log level "%s"', is_scalar($level) ? (string) $level : gettype($level)));
        }
        $this->debugger->log(Levels::PSR3[$level], self::interpolate((string) $message, $context), $context, ['psr3']);
        if (($context['exception'] ?? null) instanceof \Throwable) {
            $this->debugger->exception($context['exception']);
        }
    }

    /**
     * {placeholder} → context value, as PSR-3 describes; values that are not text stay as placeholders.
     *
     * @param array<string, mixed> $context
     */
    private static function interpolate(string $message, array $context): string
    {
        if (strpos($message, '{') === false) {
            return $message;
        }
        $replacements = [];
        foreach ($context as $key => $value) {
            if (is_scalar($value) || $value === null) {
                $replacements['{' . $key . '}'] = is_bool($value) ? ($value ? 'true' : 'false') : (string) $value;
            } elseif ($value instanceof \DateTimeInterface) {
                $replacements['{' . $key . '}'] = $value->format(DATE_ATOM);
            } elseif (is_object($value) && method_exists($value, '__toString')) {
                $replacements['{' . $key . '}'] = (string) $value;
            }
        }

        return strtr($message, $replacements);
    }
}
