<?php

declare(strict_types=1);

namespace WappDebugger\Tests;

use PHPUnit\Framework\TestCase;
use WappDebugger\Tests\Support\TempDir;

/**
 * The HTTP mode behind a real web server (php -S): what a client of the viewer will see.
 */
final class HttpModeProcessTest extends TestCase
{
    use TempDir;

    private const TOKEN = 'a-long-enough-token-of-32-chars!';

    /** @var resource|null */
    private $server;
    private int $port = 0;

    protected function setUp(): void
    {
        file_put_contents($this->tempDir() . '/dws_debugger.json', json_encode(['enabled' => false, 'logs_dir' => 'logs', 'http_token' => self::TOKEN]));
        mkdir($this->tempDir() . '/logs');
        $this->port = self::freePort();
        // index.php as the router: every request goes through the application, as behind nginx + FPM
        // (without a router php -S 7.4 answers 404 itself for a missing file with an extension, like *.jsonl)
        $command = sprintf('exec %s -S 127.0.0.1:%d %s', escapeshellarg(PHP_BINARY), $this->port, escapeshellarg(__DIR__ . '/scripts/web/index.php'));
        $server = proc_open($command, [1 => ['file', '/dev/null', 'w'], 2 => ['file', '/dev/null', 'w']], $pipes, null, ['WAPP_TEST_DIR' => $this->tempDir()] + getenv());
        self::assertIsResource($server);
        $this->server = $server;
        for ($i = 0; $i < 100; ++$i) {
            $connection = @fsockopen('127.0.0.1', $this->port);
            if ($connection !== false) {
                fclose($connection);

                return;
            }
            $status = proc_get_status($server);
            self::assertTrue(is_array($status) && $status['running'], 'php -S exited before it started listening');
            usleep(100000);
        }
        self::fail("php -S does not listen on port {$this->port} after 10 s");
    }

    /** A port the system has just handed out, so it is free (a random one may be taken). */
    private static function freePort(): int
    {
        $socket = stream_socket_server('tcp://127.0.0.1:0');
        self::assertIsResource($socket);
        $name = (string) stream_socket_get_name($socket, false);
        fclose($socket);

        return (int) substr($name, (int) strrpos($name, ':') + 1);
    }

    protected function tearDown(): void
    {
        if ($this->server !== null) {
            proc_terminate($this->server);
            proc_close($this->server);
        }
    }

    /**
     * @param list<string> $headers
     *
     * @return array{int, string}
     */
    private function request(string $method, string $path, array $headers = [], string $body = ''): array
    {
        $context = stream_context_create(['http' => ['method' => $method, 'header' => $headers, 'content' => $body, 'ignore_errors' => true, 'proxy' => '']]);
        $response = (string) file_get_contents('http://127.0.0.1:' . $this->port . $path, false, $context);
        /** @var list<string> $http_response_header set by file_get_contents */
        self::assertSame(1, preg_match('~^HTTP/\S+ (\d{3})~', $http_response_header[0], $match));

        return [(int) ($match[1] ?? 0), $response];
    }

    public function testWithoutATokenTheApplicationAnswers(): void
    {
        self::assertSame([404, 'the application answered'], $this->request('GET', '/__wapp_debugger/v1/status'));
    }

    public function testTurnsTheDebuggerOnAndListsTheNewLog(): void
    {
        $token = ['X-Debugger-Token: ' . self::TOKEN];
        [$status, $body] = $this->request('PUT', '/__wapp_debugger/v1/enabled', array_merge($token, ['Content-Type: application/json']), '{"enabled":true}');
        self::assertSame(200, $status, $body);
        self::assertTrue(json_decode($body, true)['enabled']);

        $this->request('GET', '/catalog?id=1'); // now logged
        [, $list] = $this->request('GET', '/__wapp_debugger/v1/logs', $token);
        $files = json_decode($list, true)['files'];
        self::assertCount(1, $files, 'control requests are not logged');

        [$status, $log] = $this->request('GET', '/__wapp_debugger/v1/logs/' . $files[0]['name'], $token);
        self::assertSame(200, $status);
        self::assertStringContainsString('"url":"http://127.0.0.1:' . $this->port . '/catalog?id=1"', $log);
        self::assertStringContainsString('"status_code":404', $log);
    }
}
