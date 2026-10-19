<?php

declare(strict_types=1);

namespace WappDebugger\Tests;

use PHPUnit\Framework\TestCase;
use WappDebugger\Config;
use WappDebugger\Debugger;
use WappDebugger\JsonlFileWriter;
use WappDebugger\Level;
use WappDebugger\RequestInfo;
use WappDebugger\Tests\Support\FixedClock;
use WappDebugger\Tests\Support\LogAssert;
use WappDebugger\Tests\Support\TempDir;

final class DebuggerTest extends TestCase
{
    use TempDir;

    private const START = 1792400000000;

    private function debugger(FixedClock $clock, ?RequestInfo $request = null): Debugger
    {
        $config = Config::fromArray(['enabled' => true, 'logs_dir' => $this->tempDir() . '/logs', 'project_root' => $this->tempDir()]);

        return Debugger::start($config, $request ?? RequestInfo::web('GET', 'https://shop.test/item?id=42'), $clock);
    }

    public function testWritesAValidV2Stream(): void
    {
        $clock = new FixedClock(self::START);
        $debugger = $this->debugger($clock);
        $clock->advance(2);
        $debugger->log(Level::INFO, 'Loading the item', ['item_id' => 42]);
        $clock->advance(38);
        $debugger->finish(200);

        $files = $this->logFiles($this->tempDir() . '/logs');
        self::assertCount(1, $files);
        self::assertMatchesRegularExpression('~/' . self::START . '-[0-9A-HJKMNP-TV-Z]{26}\.jsonl$~', $files[0]);

        $events = LogAssert::validStream($files[0]);
        self::assertSame(['request_start', 'message', 'request_end'], array_column($events, 'type'));
        self::assertSame($debugger->requestId(), $events[0]['request_id']);
        self::assertSame('web', $events[0]['sapi']);
        self::assertSame('php', $events[0]['lang']);
        self::assertSame(Debugger::VERSION, $events[0]['library_version']);
        self::assertSame(PHP_VERSION, $events[0]['php_version']);
        self::assertSame('GET', $events[0]['method']);
        self::assertSame('https://shop.test/item?id=42', $events[0]['url']);
        self::assertSame(['item_id' => 42], $events[1]['context']);
        self::assertSame(self::START + 2, $events[1]['ts']);
        self::assertSame(40.0, (float) $events[2]['duration_ms'], 'measured on the monotonic clock');
        self::assertSame(200, $events[2]['status_code']);
        self::assertIsInt($events[2]['memory_peak_bytes']);
    }

    public function testCliRequestHasArgvAndExitCode(): void
    {
        $debugger = $this->debugger(new FixedClock(self::START), RequestInfo::cli(['bin/console', 'cache:clear']));
        $debugger->finish(3);

        $events = LogAssert::validStream($this->logFiles($this->tempDir() . '/logs')[0]);
        self::assertSame('cli', $events[0]['sapi']);
        self::assertSame(['bin/console', 'cache:clear'], $events[0]['argv']);
        self::assertSame(3, $events[1]['exit_code']);
        self::assertArrayNotHasKey('status_code', $events[1]);
    }

    public function testContextIsAlwaysAnObject(): void
    {
        $debugger = $this->debugger(new FixedClock(self::START));
        $debugger->log(Level::MESSAGE, 'no context');
        $debugger->log(Level::MESSAGE, 'a list', [10, 20]);

        $content = (string) file_get_contents($this->logFiles($this->tempDir() . '/logs')[0]);
        $lines = explode("\n", trim($content));
        self::assertStringNotContainsString('"context"', $lines[1], 'an empty context is left out');
        self::assertStringContainsString('"context":{"0":10,"1":20}', $lines[2]);
        LogAssert::validStream($this->logFiles($this->tempDir() . '/logs')[0]);
    }

    public function testContextIsSerializedAndMasked(): void
    {
        $config = Config::fromArray([
            'enabled' => true, 'logs_dir' => $this->tempDir() . '/logs', 'mask_keys' => ['card_number'], 'max_string_bytes' => 4,
        ]);
        $debugger = Debugger::start($config, RequestInfo::cli([]), new FixedClock(self::START));
        $debugger->log(Level::INFO, 'payment', ['password' => 'p', 'card_number' => '4111', 'note' => 'abcdefgh', 'at' => new \DateTimeImmutable('@0')]);

        $events = LogAssert::validStream($this->logFiles($this->tempDir() . '/logs')[0]);
        self::assertSame(
            ['password' => '***', 'card_number' => '***', 'note' => 'abcd… (+4 bytes)', 'at' => ['__class' => 'DateTimeImmutable', 'value' => '1970-01-01T00:00:00+00:00']],
            $events[1]['context']
        );
    }

    public function testLevelShortcuts(): void
    {
        $debugger = $this->debugger(new FixedClock(self::START));
        $debugger->error('e', ['a' => 1], ['billing']);
        $debugger->warning('w');
        $debugger->info('i');
        $debugger->message('m');
        $debugger->debug('d');

        $events = array_slice(LogAssert::validStream($this->logFiles($this->tempDir() . '/logs')[0]), 1);
        self::assertSame(['error', 'warning', 'info', 'message', 'debug'], array_column($events, 'level'));
        self::assertSame(['a' => 1], $events[0]['context']);
        self::assertSame(['billing'], $events[0]['tags']);
    }

    public function testUnknownLevelBecomesMessage(): void
    {
        $debugger = $this->debugger(new FixedClock(self::START));
        $debugger->log('fatal-ish', 'logging never breaks the application');

        $events = LogAssert::validStream($this->logFiles($this->tempDir() . '/logs')[0]);
        self::assertSame('message', $events[1]['level']);
    }

    public function testTimeNeverGoesBackwards(): void
    {
        $clock = new FixedClock(self::START + 100);
        $debugger = $this->debugger($clock);
        $clock->advance(-50); // NTP correction
        $debugger->log(Level::DEBUG, 'after the clock jumped back');

        $events = LogAssert::validStream($this->logFiles($this->tempDir() . '/logs')[0]);
        self::assertSame(self::START + 100, $events[1]['ts']);
    }

    public function testNothingIsWrittenAfterFinish(): void
    {
        $debugger = $this->debugger(new FixedClock(self::START));
        $debugger->finish(200);
        $debugger->finish(500);
        $debugger->log(Level::ERROR, 'too late');

        self::assertCount(2, LogAssert::validStream($this->logFiles($this->tempDir() . '/logs')[0]));
    }

    public function testDisabledDebuggerTouchesNothing(): void
    {
        $logs = $this->tempDir() . '/logs';
        $debugger = Debugger::start(Config::fromArray(['enabled' => false, 'logs_dir' => $logs]), RequestInfo::cli([]), new FixedClock(self::START));
        $debugger->log(Level::ERROR, 'ignored');
        $debugger->finish(0);

        self::assertFalse($debugger->enabled());
        self::assertDirectoryDoesNotExist($logs);
    }

    public function testCreatesTheLogsDirectoryAndFilesWithConfiguredModes(): void
    {
        $logs = $this->tempDir() . '/deep/tmp/logs';
        $old = umask(0077);
        try {
            $debugger = Debugger::start(
                Config::fromArray(['enabled' => true, 'logs_dir' => $logs, 'file_mode' => 0664, 'dir_mode' => 0775]),
                RequestInfo::cli([]),
                new FixedClock(self::START)
            );
            $debugger->finish(0);
        } finally {
            umask($old);
        }

        self::assertSame('0775', substr(sprintf('%o', fileperms($logs)), -4));
        self::assertSame('0664', substr(sprintf('%o', fileperms($this->logFiles($logs)[0])), -4));
    }

    public function testRequestIdsAreUniqueAndSortByTime(): void
    {
        $first = $this->debugger(new FixedClock(self::START));
        $second = $this->debugger(new FixedClock(self::START + 1));

        self::assertNotSame($first->requestId(), $second->requestId());
        self::assertLessThan(0, strcmp($first->requestId(), $second->requestId()));
    }

    public function testWriterAppendsOneLinePerEvent(): void
    {
        $writer = new JsonlFileWriter($this->tempDir() . '/w', 'name.jsonl', 0664, 0775);
        $writer->write('{"a":1}');
        $writer->write('{"b":2}');

        self::assertSame("{\"a\":1}\n{\"b\":2}\n", file_get_contents($this->tempDir() . '/w/name.jsonl'));
    }
}
