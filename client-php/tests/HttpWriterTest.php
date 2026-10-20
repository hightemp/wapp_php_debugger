<?php

declare(strict_types=1);

namespace WappDebugger\Tests;

use PHPUnit\Framework\TestCase;
use WappDebugger\HttpWriter;

final class HttpWriterTest extends TestCase
{
    /** @var list<array{string, string, string}> */
    private array $sent = [];
    private bool $answer = true;

    private function writer(int $batchLines = 100): HttpWriter
    {
        return new HttpWriter('http://viewer.lan:8821/ingest/v2', 'token', function (string $url, string $token, string $body): bool {
            $this->sent[] = [$url, $token, $body];

            return $this->answer;
        }, $batchLines);
    }

    public function testSendsTheLinesInBatchesAndTheRestOnClose(): void
    {
        $writer = $this->writer(2);
        self::assertTrue($writer->write('{"seq":1}'));
        self::assertSame([], $this->sent, 'a line waits for its batch');
        self::assertTrue($writer->write('{"seq":2}'));
        self::assertTrue($writer->write('{"seq":3}'));
        $writer->close();
        $writer->close();

        self::assertSame([
            ['http://viewer.lan:8821/ingest/v2', 'token', "{\"seq\":1}\n{\"seq\":2}\n"],
            ['http://viewer.lan:8821/ingest/v2', 'token', "{\"seq\":3}\n"],
        ], $this->sent);
    }

    public function testAFailedSendStopsLogging(): void
    {
        $writer = $this->writer(1);
        $this->answer = false;

        self::assertFalse($writer->write('{"seq":1}'), 'the debugger stops rather than slow the application down');
        self::assertFalse($writer->write('{"seq":2}'));
        self::assertCount(1, $this->sent);
    }

    public function testALongRunSendsWhatItHasEverySecond(): void
    {
        $now = 1000.0;
        $writer = new HttpWriter('http://viewer.lan/ingest/v2', 'token', function (string $url, string $token, string $body): bool {
            $this->sent[] = [$url, $token, $body];

            return true;
        }, 100, static function () use (&$now): float {
            return $now;
        });
        $writer->write('{"seq":1}');
        $now += 1.5;
        $writer->write('{"seq":2}');

        self::assertCount(1, $this->sent, 'a CLI job shows up in the viewer while it runs');
    }
}
