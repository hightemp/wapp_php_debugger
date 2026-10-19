<?php

declare(strict_types=1);

namespace WappDebugger\Tests;

use PHPUnit\Framework\TestCase;
use WappDebugger\Config;
use WappDebugger\HttpEndpoint;
use WappDebugger\HttpResponse;
use WappDebugger\Tests\Support\TempDir;

final class HttpEndpointTest extends TestCase
{
    use TempDir;

    private const TOKEN = 'a-long-enough-token-of-32-chars!';
    private const LOG = '1792400000000-01JAZ7Q5X0K4M8T2V6N9P3R5S7.jsonl';

    /**
     * @param array<string, mixed> $config
     */
    private function endpoint(array $config = []): HttpEndpoint
    {
        $file = $this->tempDir() . '/dws_debugger.json';
        file_put_contents($file, json_encode($config + ['enabled' => false, 'logs_dir' => 'logs', 'http_token' => self::TOKEN]));
        if (!is_dir($this->tempDir() . '/logs')) {
            mkdir($this->tempDir() . '/logs');
        }

        return new HttpEndpoint($file, Config::fromFile($file));
    }

    /**
     * @param array<string, string> $headers
     */
    private function call(HttpEndpoint $endpoint, string $method, string $uri, array $headers = ['X-Debugger-Token' => self::TOKEN], string $body = ''): ?HttpResponse
    {
        $server = ['REQUEST_METHOD' => $method, 'REQUEST_URI' => $uri, 'REMOTE_ADDR' => '203.0.113.10'];
        foreach ($headers as $name => $value) {
            $server['HTTP_' . strtoupper(str_replace('-', '_', $name))] = $value;
        }

        return $endpoint->handle($server, $body);
    }

    /**
     * @return array<string, mixed>
     */
    private function json(?HttpResponse $response): array
    {
        self::assertNotNull($response);
        /** @var array<string, mixed> $data */
        $data = json_decode($response->body(), true, 512, JSON_THROW_ON_ERROR);

        return $data;
    }

    public function testStatus(): void
    {
        $response = $this->call($this->endpoint(), 'GET', '/__wapp_debugger/v1/status');

        self::assertNotNull($response);
        self::assertSame(200, $response->status());
        self::assertSame('application/json', $response->headers()['Content-Type']);
        $status = $this->json($response);
        self::assertFalse($status['enabled']);
        self::assertSame(2, $status['format_version']);
        self::assertSame(PHP_VERSION, $status['php_version']);
        self::assertTrue($status['logs_dir_writable']);
        self::assertIsInt($status['disk_free_bytes']);
    }

    /**
     * @dataProvider silentCases
     *
     * @param array<string, mixed>  $config
     * @param array<string, string> $headers
     */
    public function testStaysSilentWithoutAccess(array $config, array $headers, string $uri): void
    {
        self::assertNull($this->call($this->endpoint($config), 'GET', $uri, $headers), 'the request goes to the application');
    }

    /**
     * @return iterable<string, array{array<string, mixed>, array<string, string>, string}>
     */
    public static function silentCases(): iterable
    {
        $token = ['X-Debugger-Token' => self::TOKEN];
        yield 'no token' => [[], [], '/__wapp_debugger/v1/status'];
        yield 'wrong token' => [[], ['X-Debugger-Token' => str_repeat('x', 32)], '/__wapp_debugger/v1/status'];
        yield 'token in the query' => [[], [], '/__wapp_debugger/v1/status?token=' . self::TOKEN];
        yield 'short token in the config' => [['http_token' => 'short'], ['X-Debugger-Token' => 'short'], '/__wapp_debugger/v1/status'];
        yield 'no token in the config' => [['http_token' => null], $token, '/__wapp_debugger/v1/status'];
        yield 'ip not allowed' => [['http_allowed_ips' => ['198.51.100.0/24']], $token, '/__wapp_debugger/v1/status'];
        yield 'another path' => [[], $token, '/catalog/item?id=42'];
    }

    public function testIpAllowlistWithNetworks(): void
    {
        self::assertNotNull($this->call($this->endpoint(['http_allowed_ips' => ['10.0.0.1', '203.0.113.0/24']]), 'GET', '/__wapp_debugger/v1/status'));
    }

    public function testTurnsTheDebuggerOnInTheConfigFile(): void
    {
        $endpoint = $this->endpoint(['max_files' => 7]);
        $response = $this->call($endpoint, 'PUT', '/__wapp_debugger/v1/enabled', ['X-Debugger-Token' => self::TOKEN], '{"enabled": true}');

        self::assertTrue($this->json($response)['enabled']);
        $config = json_decode((string) file_get_contents($this->tempDir() . '/dws_debugger.json'), true);
        self::assertTrue($config['enabled']);
        self::assertSame(7, $config['max_files'], 'other keys stay');
    }

    public function testRejectsABadBody(): void
    {
        $response = $this->call($this->endpoint(), 'PUT', '/__wapp_debugger/v1/enabled', ['X-Debugger-Token' => self::TOKEN], '{"enabled": "yes"}');

        self::assertNotNull($response);
        self::assertSame(400, $response->status());
        self::assertSame('application/problem+json', $response->headers()['Content-Type']);
        self::assertSame(400, $this->json($response)['status']);
    }

    public function testListsAndReadsLogs(): void
    {
        $endpoint = $this->endpoint();
        file_put_contents($this->tempDir() . '/logs/' . self::LOG, "line one\nline two\n");
        file_put_contents($this->tempDir() . '/logs/notes.txt', 'not a log');

        $files = $this->json($this->call($endpoint, 'GET', '/__wapp_debugger/v1/logs'))['files'];
        self::assertCount(1, $files);
        self::assertSame(self::LOG, $files[0]['name']);
        self::assertSame(18, $files[0]['size']);
        self::assertIsInt($files[0]['modified_at']);

        $part = $this->call($endpoint, 'GET', '/__wapp_debugger/v1/logs/' . self::LOG . '?offset=5&limit=8');
        self::assertNotNull($part);
        self::assertSame(200, $part->status());
        self::assertSame('one\nline', addcslashes($part->body(), "\n"));
        self::assertSame('18', $part->headers()['X-File-Size']);
        self::assertSame('application/octet-stream', $part->headers()['Content-Type']);

        $end = $this->call($endpoint, 'GET', '/__wapp_debugger/v1/logs/' . self::LOG . '?offset=18');
        self::assertNotNull($end);
        self::assertSame('', $end->body(), 'an empty body means the end of the file');

        $past = $this->call($endpoint, 'GET', '/__wapp_debugger/v1/logs/' . self::LOG . '?offset=19');
        self::assertNotNull($past);
        self::assertSame(416, $past->status());
    }

    /**
     * @dataProvider unsafeNames
     */
    public function testServesOnlyLogFiles(string $name): void
    {
        $response = $this->call($this->endpoint(), 'GET', '/__wapp_debugger/v1/logs/' . $name);

        self::assertNotNull($response);
        self::assertSame(404, $response->status());
    }

    /**
     * @return iterable<array{string}>
     */
    public static function unsafeNames(): iterable
    {
        yield ['..%2Fdws_debugger.json'];
        yield ['notes.txt'];
        yield ['%2Fetc%2Fpasswd'];
        yield ['1792400000001-01JAZ7Q5X0K4M8T2V6N9P3R5S7.jsonl'];
    }

    public function testRemovesAndClearsLogs(): void
    {
        $endpoint = $this->endpoint();
        file_put_contents($this->tempDir() . '/logs/' . self::LOG, 'x');
        file_put_contents($this->tempDir() . '/logs/1792400000001', 'v1');
        file_put_contents($this->tempDir() . '/logs/1792400000002', 'v1');
        file_put_contents($this->tempDir() . '/logs/keep.txt', 'mine');

        $removed = $this->call($endpoint, 'DELETE', '/__wapp_debugger/v1/logs/' . self::LOG);
        self::assertNotNull($removed);
        self::assertSame(204, $removed->status());
        self::assertSame(['removed_files' => 2], $this->json($this->call($endpoint, 'DELETE', '/__wapp_debugger/v1/logs')));
        self::assertFileExists($this->tempDir() . '/logs/keep.txt');
    }

    public function testUnknownRoutesAndMethods(): void
    {
        $endpoint = $this->endpoint();
        $unknown = $this->call($endpoint, 'GET', '/__wapp_debugger/v1/nope');
        $method = $this->call($endpoint, 'POST', '/__wapp_debugger/v1/status');

        self::assertNotNull($unknown);
        self::assertNotNull($method);
        self::assertSame(404, $unknown->status());
        self::assertSame(405, $method->status());
        self::assertSame('GET', $method->headers()['Allow']);
    }

    public function testRateLimit(): void
    {
        $endpoint = $this->endpoint(['http_rate_limit' => 2]);
        $this->call($endpoint, 'GET', '/__wapp_debugger/v1/status');
        $this->call($endpoint, 'GET', '/__wapp_debugger/v1/status');
        $third = $this->call($endpoint, 'GET', '/__wapp_debugger/v1/status');

        self::assertNotNull($third);
        self::assertSame(429, $third->status());
        self::assertSame('60', $third->headers()['Retry-After']);
    }
}
