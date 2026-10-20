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
    /** @var list<string> */
    private array $logGlobals;
    private int $maxAgeDays;
    private int $maxFiles;
    private int $maxLogsBytes;
    private ?string $httpToken;
    /** @var list<string> */
    private array $httpAllowedIps;
    private int $httpRateLimit;
    /** @var array<string, string|null> */
    private array $oldKeys = [];

    /** Keys of library v1 and their names in 2.x; null means the key is no longer used. */
    private const OLD_KEYS = [
        'bIsOnFlag' => 'enabled',
        'sProjectRootPath' => 'project_root',
        'sMessagesFilesPath' => 'logs_dir',
        'sTempPath' => null,
        'iArraySizeLimit' => 'max_items',
        'iPageSize' => null,
        'bLogSessionAndCookies' => 'log_globals',
        'iLogsSizeLimit' => 'max_logs_size',
    ];

    private const GLOBALS = ['server', 'get', 'post', 'files', 'session', 'cookie'];

    /**
     * @param array<string, mixed> $values
     */
    private function __construct(array $values, ?string $baseDir)
    {
        $values = $this->upgradeOldKeys($values);
        $this->enabled = ($values['enabled'] ?? false) === true;
        $root = self::stringOrNull($values['project_root'] ?? null);
        $this->projectRoot = $root !== null ? self::resolve($root, $baseDir) : $baseDir;
        $logs = self::stringOrNull($values['logs_dir'] ?? null);
        $this->logsDir = $logs !== null
            ? self::resolve($logs, $this->projectRoot)
            : ($this->projectRoot !== null ? $this->projectRoot . '/tmp/logs' : null);
        $this->fileMode = self::mode($values['file_mode'] ?? null, 0664);
        $this->dirMode = self::mode($values['dir_mode'] ?? null, 0775);
        $this->maskKeys = is_array($values['mask_keys'] ?? null) ? array_values(array_filter($values['mask_keys'], 'is_string')) : [];
        $this->maxDepth = self::positiveInt($values['max_depth'] ?? null, 6);
        $this->maxItems = self::positiveInt($values['max_items'] ?? null, 100);
        $this->maxStringBytes = self::positiveInt($values['max_string_bytes'] ?? null, 2000);
        $traceFrames = $values['trace_frames'] ?? null;
        $this->traceFrames = is_int($traceFrames) && $traceFrames >= 0 ? $traceFrames : 10;
        $this->maxAgeDays = self::nonNegativeInt($values['max_age_days'] ?? null, 30);
        $this->maxFiles = self::nonNegativeInt($values['max_files'] ?? null, 5000);
        $this->maxLogsBytes = self::nonNegativeInt($values['max_logs_size'] ?? null, 256 * 1024 * 1024);
        $token = $values['http_token'] ?? null;
        // A short token is as good as none: the HTTP mode stays off
        $this->httpToken = is_string($token) && strlen($token) >= 32 ? $token : null;
        $this->httpAllowedIps = is_array($values['http_allowed_ips'] ?? null) ? array_values(array_filter($values['http_allowed_ips'], 'is_string')) : [];
        $this->httpRateLimit = self::positiveInt($values['http_rate_limit'] ?? null, 600);
        $globals = is_array($values['log_globals'] ?? null) ? $values['log_globals'] : ['get', 'post', 'files'];
        $globals = array_map('strtolower', array_filter($globals, 'is_string'));
        $this->logGlobals = array_values(array_intersect($globals, self::GLOBALS));
        if ($this->logsDir === null) {
            $this->enabled = false;
        }
        if (!self::environmentAllowed($values)) {
            // A server outside allowed_envs (production) stays silent: no logs, no HTTP mode to turn them on
            $this->enabled = false;
            $this->httpToken = null;
        }
    }

    /**
     * allowed_envs lists the environments the debugger may work in, by the variable env_var (APP_ENV).
     * Without the list any environment is allowed; with it, an unknown environment is not.
     *
     * @param array<string, mixed> $values
     */
    private static function environmentAllowed(array $values): bool
    {
        $allowed = $values['allowed_envs'] ?? null;
        if (is_string($allowed)) {
            $allowed = [$allowed];
        }
        if (!is_array($allowed)) {
            return true;
        }
        $allowed = array_map('strtolower', array_filter($allowed, 'is_string'));
        $name = self::stringOrNull($values['env_var'] ?? null) ?? 'APP_ENV';
        $current = getenv($name);
        if (!is_string($current) || $current === '') {
            $current = $_SERVER[$name] ?? $_ENV[$name] ?? null;
        }

        return is_string($current) && in_array(strtolower($current), $allowed, true);
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

    /** Token of the HTTP mode (at least 32 characters); null keeps the mode off. */
    public function httpToken(): ?string
    {
        return $this->httpToken;
    }

    /**
     * Addresses or networks (CIDR) allowed to use the HTTP mode; empty means any.
     *
     * @return list<string>
     */
    public function httpAllowedIps(): array
    {
        return $this->httpAllowedIps;
    }

    /** HTTP mode requests per minute. */
    public function httpRateLimit(): int
    {
        return $this->httpRateLimit;
    }

    /** Log files older than this are deleted; 0 keeps them. */
    public function maxAgeDays(): int
    {
        return $this->maxAgeDays;
    }

    /** At most this many log files; 0 means no limit. */
    public function maxFiles(): int
    {
        return $this->maxFiles;
    }

    /** At most this many bytes of logs (max_logs_size); 0 means no limit. */
    public function maxLogsBytes(): int
    {
        return $this->maxLogsBytes;
    }

    /**
     * Superglobals written at boot: server, get, post, files, session, cookie.
     *
     * @return list<string>
     */
    public function logGlobals(): array
    {
        return $this->logGlobals;
    }

    /**
     * Keys of library v1 found in the config, with their new names (null: not used any more).
     *
     * @return array<string, string|null>
     */
    public function oldKeys(): array
    {
        return $this->oldKeys;
    }

    /**
     * @param array<string, mixed> $values
     *
     * @return array<string, mixed>
     */
    private function upgradeOldKeys(array $values): array
    {
        foreach (self::OLD_KEYS as $old => $new) {
            if (!array_key_exists($old, $values)) {
                continue;
            }
            $this->oldKeys[$old] = $new;
            if ($new === 'max_logs_size') {
                if (is_int($values[$old]) && !array_key_exists($new, $values)) {
                    $values[$new] = $values[$old] * 1024; // v1 counted kilobytes
                }
            } elseif ($new === 'log_globals') {
                if ($values[$old] === true && !array_key_exists('log_globals', $values)) {
                    $values['log_globals'] = ['get', 'post', 'files', 'session', 'cookie'];
                }
            } elseif ($new !== null && !array_key_exists($new, $values)) {
                $values[$new] = $values[$old];
            }
        }

        return $values;
    }

    /**
     * A permission mode: an int from PHP code or an octal string ("0664") from JSON, which has no octal numbers.
     *
     * @param mixed $value
     */
    private static function mode($value, int $default): int
    {
        if (is_int($value) && $value >= 0 && $value <= 0777) {
            return $value;
        }

        return is_string($value) && preg_match('/^0?[0-7]{3}$/', $value) === 1 ? (int) octdec($value) : $default;
    }

    /**
     * @param mixed $value
     */
    private static function nonNegativeInt($value, int $default): int
    {
        return is_int($value) && $value >= 0 ? $value : $default;
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
