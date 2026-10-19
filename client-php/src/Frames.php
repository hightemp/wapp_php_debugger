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
    private ?string $projectRoot;
    private string $libraryDir;

    public function __construct(?string $projectRoot)
    {
        $this->projectRoot = $projectRoot !== null ? rtrim($projectRoot, '/') . '/' : null;
        $this->libraryDir = __DIR__ . DIRECTORY_SEPARATOR;
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
     * @param list<array<string, mixed>> $backtrace
     *
     * @return array{source: array<string, mixed>|null, trace: list<array<string, mixed>>}
     */
    public function fromBacktrace(array $backtrace): array
    {
        // The first frame whose call site is outside the library is the call into the library
        $callIndex = null;
        foreach ($backtrace as $index => $frame) {
            if (!isset($frame['file']) || !is_string($frame['file']) || strpos($frame['file'], $this->libraryDir) !== 0) {
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

    private function relative(string $file): string
    {
        return $this->projectRoot !== null && strpos($file, $this->projectRoot) === 0 ? substr($file, strlen($this->projectRoot)) : $file;
    }
}
