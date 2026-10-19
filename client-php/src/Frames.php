<?php

declare(strict_types=1);

namespace WappDebugger;

/**
 * Finds where the application called the library (source) and the stack above it (trace).
 * Frames inside the library directory are skipped, so the global helpers and adapters
 * report the line in the application, not their own.
 */
final class Frames
{
    /** @var array<string, true> directories of logging code (PSR-3, Monolog) whose frames are skipped too */
    private static array $loggingDirs = [];

    /** true in the one-file build (tools/bin/bundle): then this file is the library, not its directory. */
    private const BUNDLED = false;

    private ?string $projectRoot;
    private string $libraryDir;

    public function __construct(?string $projectRoot)
    {
        $this->projectRoot = $projectRoot !== null ? rtrim($projectRoot, '/') . '/' : null;
        $this->libraryDir = __DIR__ . DIRECTORY_SEPARATOR;
    }

    /** Frames from this directory are logging code, not the application: used by the bridges. */
    public static function skipDirectory(string $dir): void
    {
        self::$loggingDirs[rtrim($dir, '/\\') . DIRECTORY_SEPARATOR] = true;
    }

    /**
     * @return array{source: array<string, mixed>|null, trace: list<array<string, mixed>>}
     */
    public function callStack(): array
    {
        return $this->fromBacktrace(debug_backtrace(DEBUG_BACKTRACE_IGNORE_ARGS));
    }

    /**
     * Frames of an exception: its own file and line, then its trace.
     *
     * @return array{source: array<string, mixed>, trace: list<array<string, mixed>>}
     */
    public function ofThrowable(\Throwable $throwable): array
    {
        $trace = [];
        foreach ($throwable->getTrace() as $frame) {
            $trace[] = $this->frame($frame);
        }

        return ['source' => $this->frame(['file' => $throwable->getFile(), 'line' => $throwable->getLine()]), 'trace' => $trace];
    }

    /**
     * @param array<string, mixed> $frame
     *
     * @return array<string, mixed>
     */
    public function convert(array $frame): array
    {
        return $this->frame($frame);
    }

    /**
     * @param list<array<string, mixed>> $backtrace
     *
     * @return array{source: array<string, mixed>|null, trace: list<array<string, mixed>>}
     */
    public function fromBacktrace(array $backtrace): array
    {
        // The first frame whose call site is outside the library is the call into the library
        $callIndex = null;
        foreach ($backtrace as $index => $frame) {
            if (!isset($frame['file']) || !is_string($frame['file']) || !$this->isLoggingCode($frame['file'])) {
                $callIndex = $index;
                break;
            }
        }
        if ($callIndex === null || !isset($backtrace[$callIndex]['file'])) {
            return ['source' => null, 'trace' => []];
        }

        $call = $backtrace[$callIndex];
        $enclosing = $backtrace[$callIndex + 1] ?? [];
        $source = $this->frame(['file' => $call['file'], 'line' => $call['line'] ?? 0]
            + array_intersect_key($enclosing, ['function' => true, 'class' => true, 'type' => true]));

        $trace = [];
        foreach (array_slice($backtrace, $callIndex + 1) as $frame) {
            $trace[] = $this->frame($frame);
        }

        return ['source' => $source, 'trace' => $trace];
    }

    /**
     * @param array<string, mixed> $frame a debug_backtrace() frame
     *
     * @return array<string, mixed> a frame of format v2
     */
    private function frame(array $frame): array
    {
        $result = [];
        if (isset($frame['file']) && is_string($frame['file'])) {
            $result['file'] = $this->relative($frame['file']);
        }
        if (isset($frame['line']) && is_int($frame['line'])) {
            $result['line'] = $frame['line'];
        }
        if (isset($frame['function']) && is_string($frame['function'])) {
            $result['function'] = $frame['function'];
        }
        if (isset($frame['class']) && is_string($frame['class'])) {
            $result['class'] = $frame['class'];
        }
        if (isset($frame['type']) && ($frame['type'] === '->' || $frame['type'] === '::')) {
            $result['call_type'] = $frame['type'];
        }

        return $result;
    }

    private function isLoggingCode(string $file): bool
    {
        // @phpstan-ignore booleanAnd.leftAlwaysFalse (BUNDLED is rewritten by the bundler)
        if (self::BUNDLED && $file === __FILE__) {
            return true;
        }
        // @phpstan-ignore booleanNot.alwaysTrue (BUNDLED is rewritten by the bundler)
        if (!self::BUNDLED && strpos($file, $this->libraryDir) === 0) {
            return true;
        }
        foreach (self::$loggingDirs as $dir => $_) {
            if (strpos($file, $dir) === 0) {
                return true;
            }
        }

        return false;
    }

    private function relative(string $file): string
    {
        return $this->projectRoot !== null && strpos($file, $this->projectRoot) === 0 ? substr($file, strlen($this->projectRoot)) : $file;
    }
}
