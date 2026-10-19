<?php

declare(strict_types=1);

namespace WappDebugger;

/**
 * What is known about the request or process at its start: the fields of request_start.
 */
final class RequestInfo
{
    /** @var array<string, mixed> */
    private array $fields;

    /**
     * @param array<string, mixed> $fields
     */
    private function __construct(array $fields)
    {
        $this->fields = $fields;
    }

    public static function web(string $method, string $url, bool $isAjax = false): self
    {
        return new self(['sapi' => 'web', 'method' => strtoupper($method), 'url' => $url, 'is_ajax' => $isAjax]);
    }

    /**
     * @param array<array-key, mixed> $argv
     */
    public static function cli(array $argv): self
    {
        return new self(['sapi' => 'cli', 'argv' => array_values(array_map('strval', $argv))]);
    }

    /** Header (web) and environment variable (child processes) that carry the id of the parent request. */
    public const PARENT_HEADER = 'X-Wapp-Debugger-Parent';
    public const PARENT_ENV = 'WAPP_DEBUGGER_PARENT';

    /**
     * @param array<string, mixed>       $server $_SERVER
     * @param array<string, string>|null $env    the process environment; null reads getenv()
     */
    public static function fromGlobals(array $server, string $sapi = PHP_SAPI, ?array $env = null): self
    {
        if ($sapi === 'cli' || $sapi === 'phpdbg') {
            $env ??= getenv();
            $parent = $env[self::PARENT_ENV] ?? null;
            $argv = is_array($server['argv'] ?? null) ? $server['argv'] : [];
            $info = self::cli($argv);
        } else {
            $parent = $server['HTTP_' . strtoupper(str_replace('-', '_', self::PARENT_HEADER))] ?? null;
            $https = ($server['HTTPS'] ?? '') !== '' && ($server['HTTPS'] ?? '') !== 'off';
            $host = (string) ($server['HTTP_HOST'] ?? $server['SERVER_NAME'] ?? 'localhost');
            $method = (string) ($server['REQUEST_METHOD'] ?? 'GET');
            $info = self::web(
                preg_match('/^[A-Za-z]+$/', $method) === 1 ? $method : 'GET',
                ($https ? 'https://' : 'http://') . $host . (string) ($server['REQUEST_URI'] ?? '/'),
                strtolower((string) ($server['HTTP_X_REQUESTED_WITH'] ?? '')) === 'xmlhttprequest'
            );
            $info->fields += array_filter([
                'ip' => $server['REMOTE_ADDR'] ?? null,
                'server_ip' => $server['SERVER_ADDR'] ?? null,
                'server_name' => $server['SERVER_NAME'] ?? null,
            ], 'is_string');
        }
        if (is_string($parent) && preg_match('/^[0-7][0-9A-HJKMNP-TV-Z]{25}$/', $parent) === 1) {
            $info->fields['parent_request_id'] = $parent;
        }
        $hostname = gethostname();
        if (is_string($hostname)) {
            $info->fields['hostname'] = $hostname;
        }
        $pid = getmypid();
        if (is_int($pid)) {
            $info->fields['pid'] = $pid;
        }

        return $info;
    }

    public function isCli(): bool
    {
        return $this->fields['sapi'] === 'cli';
    }

    /**
     * @return array<string, mixed>
     */
    public function fields(): array
    {
        return $this->fields;
    }
}
