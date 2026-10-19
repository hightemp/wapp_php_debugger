<?php

declare(strict_types=1);

namespace WappDebugger;

/**
 * Library settings: dws_debugger.json next to the project or an array.
 * A missing or broken file means "disabled": the debugger must never break the application.
 */
final class Config
{
    private bool $enabled;
    private ?string $projectRoot;
    private ?string $logsDir;
    private int $fileMode;
    private int $dirMode;
    /** @var list<string> */
    private array $maskKeys;
    private int $maxDepth;
    private int $maxItems;
    private int $maxStringBytes;
    private int $traceFrames;

    /**
     * @param array<string, mixed> $values
     */
    private function __construct(array $values, ?string $baseDir)
    {
        $this->enabled = ($values['enabled'] ?? false) === true;
        $root = self::stringOrNull($values['project_root'] ?? null);
        $this->projectRoot = $root !== null ? self::resolve($root, $baseDir) : $baseDir;
        $logs = self::stringOrNull($values['logs_dir'] ?? null);
        $this->logsDir = $logs !== null
            ? self::resolve($logs, $this->projectRoot)
            : ($this->projectRoot !== null ? $this->projectRoot . '/tmp/logs' : null);
        $this->fileMode = is_int($values['file_mode'] ?? null) ? $values['file_mode'] : 0664;
        $this->dirMode = is_int($values['dir_mode'] ?? null) ? $values['dir_mode'] : 0775;
        $this->maskKeys = is_array($values['mask_keys'] ?? null) ? array_values(array_filter($values['mask_keys'], 'is_string')) : [];
        $this->maxDepth = self::positiveInt($values['max_depth'] ?? null, 6);
        $this->maxItems = self::positiveInt($values['max_items'] ?? null, 100);
        $this->maxStringBytes = self::positiveInt($values['max_string_bytes'] ?? null, 2000);
        $traceFrames = $values['trace_frames'] ?? null;
        $this->traceFrames = is_int($traceFrames) && $traceFrames >= 0 ? $traceFrames : 10;
        if ($this->logsDir === null) {
            $this->enabled = false;
        }
    }

    /**
     * @param array<string, mixed> $values
     */
    public static function fromArray(array $values): self
    {
        return new self($values, null);
    }

    /** Relative paths in the file are resolved against its directory. */
    public static function fromFile(string $path): self
    {
        $baseDir = dirname($path);
        if (!is_file($path)) {
            return new self([], $baseDir);
        }
        $values = json_decode((string) @file_get_contents($path), true);

        return new self(is_array($values) ? $values : [], $baseDir);
    }

    public function enabled(): bool
    {
        return $this->enabled;
    }

    public function projectRoot(): ?string
    {
        return $this->projectRoot;
    }

    public function logsDir(): ?string
    {
        return $this->logsDir;
    }

    public function fileMode(): int
    {
        return $this->fileMode;
    }

    public function dirMode(): int
    {
        return $this->dirMode;
    }

    /**
     * @return list<string> keys to mask on top of the defaults
     */
    public function maskKeys(): array
    {
        return $this->maskKeys;
    }

    public function maxDepth(): int
    {
        return $this->maxDepth;
    }

    public function maxItems(): int
    {
        return $this->maxItems;
    }

    public function maxStringBytes(): int
    {
        return $this->maxStringBytes;
    }

    /** Frames of the trace of a message; 0 means only the source. */
    public function traceFrames(): int
    {
        return $this->traceFrames;
    }

    /**
     * @param mixed $value
     */
    private static function positiveInt($value, int $default): int
    {
        return is_int($value) && $value > 0 ? $value : $default;
    }

    /**
     * @param mixed $value
     */
    private static function stringOrNull($value): ?string
    {
        return is_string($value) && $value !== '' ? rtrim($value, '/') : null;
    }

    private static function resolve(string $path, ?string $baseDir): string
    {
        if ($path[0] === '/' || preg_match('~^[A-Za-z]:[\\\\/]~', $path) === 1 || $baseDir === null) {
            return $path;
        }

        return $baseDir . '/' . $path;
    }
}
