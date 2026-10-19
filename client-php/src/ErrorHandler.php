<?php

declare(strict_types=1);

namespace WappDebugger;

/**
 * Writes PHP errors, uncaught exceptions and fatal errors as error events and finishes the request
 * at shutdown. It never hides anything: handlers of the application keep working, and errors PHP
 * would print are still printed.
 */
final class ErrorHandler
{
    private const FATAL_TYPES = E_ERROR | E_PARSE | E_CORE_ERROR | E_COMPILE_ERROR;
    private const RESERVE_BYTES = 65536;

    private const NAMES = [
        E_ERROR => 'E_ERROR', E_WARNING => 'E_WARNING', E_PARSE => 'E_PARSE', E_NOTICE => 'E_NOTICE',
        E_CORE_ERROR => 'E_CORE_ERROR', E_CORE_WARNING => 'E_CORE_WARNING', E_COMPILE_ERROR => 'E_COMPILE_ERROR',
        E_COMPILE_WARNING => 'E_COMPILE_WARNING', E_USER_ERROR => 'E_USER_ERROR', E_USER_WARNING => 'E_USER_WARNING',
        E_USER_NOTICE => 'E_USER_NOTICE', 2048 => 'E_STRICT', E_RECOVERABLE_ERROR => 'E_RECOVERABLE_ERROR',
        E_DEPRECATED => 'E_DEPRECATED', E_USER_DEPRECATED => 'E_USER_DEPRECATED',
    ];

    private Debugger $debugger;
    /** @var callable(): ?int */
    private $responseCode;
    /** @var callable|null */
    private $previousErrorHandler;
    /** @var callable|null */
    private $previousExceptionHandler;
    private bool $uncaught = false;
    // @phpstan-ignore property.onlyWritten (memory set aside for a fatal error; released at shutdown)
    private ?string $reserve = null;

    /**
     * @param callable(): ?int $responseCode the HTTP status of a web request at shutdown
     */
    public function __construct(Debugger $debugger, callable $responseCode)
    {
        $this->debugger = $debugger;
        $this->responseCode = $responseCode;
    }

    public function register(): void
    {
        $this->reserve = str_repeat(' ', self::RESERVE_BYTES);
        $this->chainErrorHandler(set_error_handler([$this, 'handleError']));
        $this->previousExceptionHandler = set_exception_handler([$this, 'handleException']);
        register_shutdown_function(function (): void {
            $this->handleShutdown(error_get_last());
        });
    }

    public function chainErrorHandler(?callable $previous): void
    {
        $this->previousErrorHandler = $previous;
    }

    public function handleError(int $type, string $message, string $file = '', int $line = 0): bool
    {
        if ((error_reporting() & $type) !== 0) {
            $this->debugger->phpError([
                'kind' => 'php_error',
                'level' => self::level($type),
                'message' => $message,
                'error_type' => self::NAMES[$type] ?? 'E_UNKNOWN',
            ], ['file' => $file, 'line' => $line], array_slice(debug_backtrace(DEBUG_BACKTRACE_IGNORE_ARGS), 1));
        }
        if ($this->previousErrorHandler !== null) {
            return (bool) ($this->previousErrorHandler)($type, $message, $file, $line);
        }

        // PHP goes on with its own handling: the error is printed or logged as usual
        return false;
    }

    public function handleException(\Throwable $exception): void
    {
        $this->logException($exception, true);
        if ($this->previousExceptionHandler !== null) {
            ($this->previousExceptionHandler)($exception);

            return;
        }
        // Rethrowing from an exception handler does not reach PHP's own reporting (PHP 8 calls the handler
        // again), so report it the way PHP does and exit with its code
        self::reportUncaught($exception);

        exit(255);
    }

    /** What PHP prints and logs for an uncaught exception, honoring log_errors, display_errors and html_errors. */
    public static function reportUncaught(\Throwable $exception): void
    {
        $message = sprintf("Uncaught %s\n  thrown", (string) $exception);
        if (filter_var(ini_get('log_errors'), FILTER_VALIDATE_BOOLEAN)) {
            error_log(sprintf('PHP Fatal error:  %s in %s on line %d', $message, $exception->getFile(), $exception->getLine()));
        }
        $display = strtolower((string) ini_get('display_errors'));
        if ($display === 'stderr' && defined('STDERR')) {
            fwrite(STDERR, sprintf('%sFatal error: %s in %s on line %d%s', PHP_EOL, $message, $exception->getFile(), $exception->getLine(), PHP_EOL));
        } elseif ($display === 'stdout' || filter_var($display, FILTER_VALIDATE_BOOLEAN)) {
            if (filter_var(ini_get('html_errors'), FILTER_VALIDATE_BOOLEAN) && PHP_SAPI !== 'cli') {
                printf(
                    "<br />\n<b>Fatal error</b>:  %s in <b>%s</b> on line <b>%d</b><br />\n",
                    nl2br(htmlspecialchars($message, ENT_QUOTES)),
                    htmlspecialchars($exception->getFile(), ENT_QUOTES),
                    $exception->getLine()
                );
            } else {
                printf('%sFatal error: %s in %s on line %d%s', PHP_EOL, $message, $exception->getFile(), $exception->getLine(), PHP_EOL);
            }
        }
        if (PHP_SAPI !== 'cli' && !headers_sent()) {
            http_response_code(500);
        }
    }

    public function logException(\Throwable $exception, bool $uncaught): void
    {
        if ($uncaught) {
            $this->uncaught = true;
        }
        $this->debugger->exception($exception);
    }

    /**
     * @param array{type: int, message: string, file: string, line: int}|null $lastError error_get_last()
     */
    public function handleShutdown(?array $lastError): void
    {
        $this->reserve = null;
        $fatal = $lastError !== null && ($lastError['type'] & self::FATAL_TYPES) !== 0;
        if ($fatal) {
            if (strpos($lastError['message'], 'Allowed memory size') === 0) {
                self::makeRoomInMemory();
            }
            $this->debugger->phpError([
                'kind' => 'fatal',
                'level' => Level::ERROR,
                'message' => $lastError['message'],
                'error_type' => self::NAMES[$lastError['type']] ?? 'E_ERROR',
            ], ['file' => $lastError['file'], 'line' => $lastError['line']], []);
        }

        $crashed = $fatal || $this->uncaught;
        if ($this->debugger->isCli()) {
            $this->debugger->finish($crashed ? 255 : null);
        } else {
            $this->debugger->finish($crashed ? 500 : ($this->responseCode)());
        }
    }

    /** After running out of memory: a little more room to write the last events; the limit is only ever raised. */
    private static function makeRoomInMemory(): void
    {
        $limit = self::bytes((string) ini_get('memory_limit'));
        $wanted = memory_get_usage(true) + 16 * 1024 * 1024;
        if ($limit > 0 && $limit < $wanted) {
            @ini_set('memory_limit', (string) $wanted);
        }
    }

    /** memory_limit in bytes ("128M", "1G"); -1 means no limit. */
    private static function bytes(string $value): int
    {
        $value = trim($value);
        $number = (int) $value;
        switch (strtoupper(substr($value, -1))) {
            case 'G':
                return $number * 1024 * 1024 * 1024;
            case 'M':
                return $number * 1024 * 1024;
            case 'K':
                return $number * 1024;
            default:
                return $number;
        }
    }

    private static function level(int $type): string
    {
        if (($type & (E_WARNING | E_CORE_WARNING | E_COMPILE_WARNING | E_USER_WARNING)) !== 0) {
            return Level::WARNING;
        }
        if (($type & (E_NOTICE | E_USER_NOTICE | E_DEPRECATED | E_USER_DEPRECATED | 2048)) !== 0) {
            return Level::INFO;
        }

        return Level::ERROR;
    }
}
