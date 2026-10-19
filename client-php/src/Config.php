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
