<?php

declare(strict_types=1);

namespace WappDebugger;

/**
 * Optional HTTP mode (spec/library-http.yaml) for servers without SSH. It answers only requests
 * under /__wapp_debugger/v1/ with the right token from an allowed address; anything else gets null
 * and goes to the application as usual, so without the token the mode is invisible.
 */
final class HttpEndpoint
{
    public const PREFIX = '/__wapp_debugger/v1/';
    private const READ_LIMIT_DEFAULT = 1048576;
    private const READ_LIMIT_MAX = 8388608;

    private string $configPath;
    private Config $config;

    public function __construct(string $configPath, Config $config)
    {
        $this->configPath = $configPath;
        $this->config = $config;
    }

    /**
     * @param array<string, mixed> $server $_SERVER
     * @param string|null          $body   the request body; null reads php://input when needed
     */
    public function handle(array $server, ?string $body = null): ?HttpResponse
    {
        $uri = (string) ($server['REQUEST_URI'] ?? '');
        $path = (string) parse_url($uri, PHP_URL_PATH);
        if (strpos($path, self::PREFIX) !== 0 || $this->config->logsDir() === null || !$this->authorized($server)) {
            return null;
        }
        if (!$this->withinRateLimit()) {
            return HttpResponse::problem(429, 'Too Many Requests', 'the HTTP mode rate limit is reached', ['Retry-After' => '60']);
        }

        parse_str((string) parse_url($uri, PHP_URL_QUERY), $query);
        $method = strtoupper((string) ($server['REQUEST_METHOD'] ?? 'GET'));
        $route = substr($path, strlen(self::PREFIX));

        if ($route === 'status') {
            return $method === 'GET' ? $this->status() : self::notAllowed('GET');
        }
        if ($route === 'enabled') {
            return $method === 'PUT' ? $this->setEnabled($body ?? (string) file_get_contents('php://input')) : self::notAllowed('PUT');
        }
        if ($route === 'logs') {
            if ($method === 'GET') {
                return $this->listLogs();
            }

            return $method === 'DELETE' ? $this->clearLogs() : self::notAllowed('GET, DELETE');
        }
        if (strpos($route, 'logs/') === 0) {
            $name = rawurldecode(substr($route, 5));
            if (preg_match(Rotation::LOG_NAME, $name) !== 1 || !is_file($this->logsDir() . '/' . $name)) {
                return HttpResponse::problem(404, 'Not Found', 'no such log file');
            }
            if ($method === 'GET') {
                return $this->readLog($name, $query);
            }

            return $method === 'DELETE' ? $this->removeLog($name) : self::notAllowed('GET, DELETE');
        }

        return HttpResponse::problem(404, 'Not Found', 'no such endpoint');
    }

    /**
     * @param array<string, mixed> $server
     */
    private function authorized(array $server): bool
    {
        $token = $this->config->httpToken();
        $given = $server['HTTP_X_DEBUGGER_TOKEN'] ?? null;
        if ($token === null || !is_string($given) || !hash_equals($token, $given)) {
            return false;
        }
        $allowed = $this->config->httpAllowedIps();
        if ($allowed === []) {
            return true;
        }
        $ip = (string) ($server['REMOTE_ADDR'] ?? '');
        foreach ($allowed as $rule) {
            if (self::ipMatches($ip, $rule)) {
                return true;
            }
        }

        return false;
    }

    /** An address or a network in CIDR notation, IPv4 or IPv6. */
    private static function ipMatches(string $ip, string $rule): bool
    {
        $slash = strpos($rule, '/');
        $network = $slash === false ? $rule : substr($rule, 0, $slash);
        $address = @inet_pton($ip);
        $networkAddress = @inet_pton($network);
        if ($address === false || $networkAddress === false || strlen($address) !== strlen($networkAddress)) {
            return false;
        }
        $bits = $slash === false ? strlen($address) * 8 : min(strlen($address) * 8, max(0, (int) substr($rule, $slash + 1)));
        $bytes = intdiv($bits, 8);
        if (strncmp($address, $networkAddress, $bytes) !== 0) {
            return false;
        }
        $rest = $bits % 8;
        if ($rest === 0) {
            return true;
        }
        $mask = (0xFF << (8 - $rest)) & 0xFF;

        return (ord($address[$bytes]) & $mask) === (ord($networkAddress[$bytes]) & $mask);
    }

    /** A fixed one-minute window shared by all clients, counted in a file next to the logs. */
    private function withinRateLimit(): bool
    {
        $handle = @fopen($this->logsDir() . '/.http-rate', 'c+');
        if ($handle === false) {
            return true;
        }
        try {
            flock($handle, LOCK_EX);
            $state = json_decode((string) stream_get_contents($handle), true);
            $window = intdiv(time(), 60);
            $count = is_array($state) && ($state['window'] ?? null) === $window ? (int) ($state['count'] ?? 0) : 0;
            if ($count >= $this->config->httpRateLimit()) {
                return false;
            }
            ftruncate($handle, 0);
            rewind($handle);
            fwrite($handle, (string) json_encode(['window' => $window, 'count' => $count + 1]));

            return true;
        } finally {
            flock($handle, LOCK_UN);
            fclose($handle);
        }
    }

    private function status(): HttpResponse
    {
        $dir = $this->logsDir();
        $free = @disk_free_space(is_dir($dir) ? $dir : dirname($dir));

        return HttpResponse::json(200, [
            'enabled' => $this->config->enabled(),
            'library_version' => Debugger::VERSION,
            'format_version' => 2,
            'php_version' => PHP_VERSION,
            'logs_dir_writable' => is_dir($dir) ? is_writable($dir) : is_writable(dirname($dir)),
            'disk_free_bytes' => $free === false ? null : (int) $free,
        ]);
    }

    private function setEnabled(string $body): HttpResponse
    {
        $request = json_decode($body, true);
        if (!is_array($request) || !is_bool($request['enabled'] ?? null) || count($request) !== 1) {
            return HttpResponse::problem(400, 'Bad Request', 'expected {"enabled": true|false}');
        }
        $values = is_file($this->configPath) ? json_decode((string) file_get_contents($this->configPath), true) : [];
        if (!is_array($values)) {
            return HttpResponse::problem(500, 'Internal Server Error', 'dws_debugger.json is not valid JSON');
        }
        $values['enabled'] = $request['enabled'];
        // In place, so the owner and the mode of the file stay (PHP and the viewer may be different users)
        $written = @file_put_contents($this->configPath, json_encode($values, JSON_PRETTY_PRINT | JSON_UNESCAPED_SLASHES | JSON_UNESCAPED_UNICODE) . "\n", LOCK_EX);
        if ($written === false) {
            return HttpResponse::problem(500, 'Internal Server Error', 'cannot write dws_debugger.json');
        }
        $this->config = Config::fromFile($this->configPath);

        return $this->status();
    }

    private function listLogs(): HttpResponse
    {
        $files = [];
        foreach ($this->logNames() as $name) {
            $path = $this->logsDir() . '/' . $name;
            $files[] = ['name' => $name, 'size' => (int) @filesize($path), 'modified_at' => (int) @filemtime($path) * 1000];
        }

        return HttpResponse::json(200, ['files' => $files]);
    }

    /**
     * @param array<array-key, mixed> $query
     */
    private function readLog(string $name, array $query): HttpResponse
    {
        $offset = self::intParam($query, 'offset', 0);
        $limit = self::intParam($query, 'limit', self::READ_LIMIT_DEFAULT);
        if ($offset === null || $offset < 0 || $limit === null || $limit < 1 || $limit > self::READ_LIMIT_MAX) {
            return HttpResponse::problem(400, 'Bad Request', sprintf('offset must be >= 0 and limit between 1 and %d', self::READ_LIMIT_MAX));
        }
        $path = $this->logsDir() . '/' . $name;
        clearstatcache(true, $path);
        $size = (int) @filesize($path);
        if ($offset > $size) {
            return HttpResponse::problem(416, 'Range Not Satisfiable', 'offset is past the end of the file: it was truncated or replaced');
        }
        $handle = @fopen($path, 'rb');
        if ($handle === false) {
            return HttpResponse::problem(500, 'Internal Server Error', 'cannot read the log file');
        }
        fseek($handle, $offset);
        $data = $offset < $size ? (string) fread($handle, max(1, min($limit, $size - $offset))) : '';
        fclose($handle);

        return new HttpResponse(200, ['Content-Type' => 'application/octet-stream', 'X-File-Size' => (string) $size], $data);
    }

    private function removeLog(string $name): HttpResponse
    {
        if (!@unlink($this->logsDir() . '/' . $name)) {
            return HttpResponse::problem(500, 'Internal Server Error', 'cannot delete the log file');
        }

        return new HttpResponse(204, [], '');
    }

    private function clearLogs(): HttpResponse
    {
        $removed = 0;
        foreach ($this->logNames() as $name) {
            if (@unlink($this->logsDir() . '/' . $name)) {
                ++$removed;
            }
        }

        return HttpResponse::json(200, ['removed_files' => $removed]);
    }

    /**
     * @return list<string>
     */
    private function logNames(): array
    {
        $names = @scandir($this->logsDir());
        if ($names === false) {
            return [];
        }

        return array_values(array_filter($names, static function (string $name): bool {
            return preg_match(Rotation::LOG_NAME, $name) === 1;
        }));
    }

    private function logsDir(): string
    {
        return (string) $this->config->logsDir();
    }

    /**
     * @param array<array-key, mixed> $query
     */
    private static function intParam(array $query, string $name, int $default): ?int
    {
        if (!isset($query[$name])) {
            return $default;
        }

        return is_string($query[$name]) && preg_match('/^\d{1,18}$/', $query[$name]) === 1 ? (int) $query[$name] : null;
    }

    private static function notAllowed(string $allow): HttpResponse
    {
        return HttpResponse::problem(405, 'Method Not Allowed', 'use ' . $allow, ['Allow' => $allow]);
    }
}
