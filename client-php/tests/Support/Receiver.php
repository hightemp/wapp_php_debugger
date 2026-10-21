<?php

declare(strict_types=1);

namespace WappDebugger\Tests\Support;

/**
 * A receiver of pushed logs (tests/scripts/receiver.php under php -S) for a test: what the library
 * pushes lands in pushed.jsonl and headers.txt of the temporary directory.
 */
trait Receiver
{
    /**
     * Runs $test with the URL of the receiver, then stops it.
     *
     * @param callable(string): void $test
     */
    protected function withReceiver(string $dir, callable $test): void
    {
        $socket = stream_socket_server('tcp://127.0.0.1:0');
        self::assertIsResource($socket);
        $name = (string) stream_socket_get_name($socket, false);
        fclose($socket);
        $port = (int) substr($name, (int) strrpos($name, ':') + 1);
        $command = sprintf('exec %s -S 127.0.0.1:%d %s', escapeshellarg(PHP_BINARY), $port, escapeshellarg(dirname(__DIR__) . '/scripts/receiver.php'));
        $receiver = proc_open($command, [1 => ['file', '/dev/null', 'w'], 2 => ['file', '/dev/null', 'w']], $pipes, null, ['WAPP_TEST_DIR' => $dir] + getenv());
        self::assertIsResource($receiver);
        try {
            for ($i = 0; $i < 100 && @fsockopen('127.0.0.1', $port) === false; ++$i) {
                usleep(100000);
            }
            $test("http://127.0.0.1:{$port}/ingest/v2");
        } finally {
            proc_terminate($receiver);
            proc_close($receiver);
        }
    }
}
