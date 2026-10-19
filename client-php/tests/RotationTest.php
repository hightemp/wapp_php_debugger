<?php

declare(strict_types=1);

namespace WappDebugger\Tests;

use PHPUnit\Framework\TestCase;
use WappDebugger\Config;
use WappDebugger\Debugger;
use WappDebugger\RequestInfo;
use WappDebugger\Rotation;
use WappDebugger\Tests\Support\FixedClock;
use WappDebugger\Tests\Support\TempDir;

final class RotationTest extends TestCase
{
    use TempDir;

    private const NOW = 1792400000000;
    private const DAY = 86400000;

    private function logs(): string
    {
        $dir = $this->tempDir() . '/logs';
        if (!is_dir($dir)) {
            mkdir($dir);
        }

        return $dir;
    }

    private function log(int $ts, int $bytes = 10, string $id = '01JAZ7Q5X0K4M8T2V6N9P3R5S7'): string
    {
        $name = $ts . '-' . $id . '.jsonl';
        file_put_contents($this->logs() . '/' . $name, str_repeat('x', $bytes));

        return $name;
    }

    /**
     * @return list<string>
     */
    private function names(): array
    {
        $names = array_values(array_diff((array) scandir($this->logs()), ['.', '..', '.rotation']));
        sort($names);

        return array_map('strval', $names);
    }

    public function testDeletesFilesOlderThanTheMaxAge(): void
    {
        $old = $this->log(self::NOW - 31 * self::DAY);
        $fresh = $this->log(self::NOW - 29 * self::DAY);

        (new Rotation(30, 0, 0))->run($this->logs(), self::NOW);

        self::assertSame([$fresh], $this->names());
        self::assertNotContains($old, $this->names());
    }

    public function testKeepsTheNewestFiles(): void
    {
        $names = [];
        for ($i = 1; $i <= 5; ++$i) {
            $names[] = $this->log(self::NOW - $i * 1000);
        }

        (new Rotation(0, 3, 0))->run($this->logs(), self::NOW);

        $expected = array_slice($names, 0, 3);
        sort($expected);
        self::assertSame($expected, $this->names());
    }

    public function testKeepsTheTotalSizeUnderTheLimit(): void
    {
        $this->log(self::NOW - 3000, 400);
        $middle = $this->log(self::NOW - 2000, 400);
        $newest = $this->log(self::NOW - 1000, 400);

        (new Rotation(0, 0, 1000))->run($this->logs(), self::NOW);

        self::assertSame([$middle, $newest], $this->names());
    }

    public function testTouchesOnlyLogFilesAndNeverTheCurrentOne(): void
    {
        file_put_contents($this->logs() . '/notes.txt', 'mine');
        file_put_contents($this->logs() . '/1700000000000', 'v1 log');
        $current = $this->log(1700000000001);

        (new Rotation(1, 0, 0))->run($this->logs(), self::NOW, $current);

        self::assertSame([$current, 'notes.txt'], $this->names(), 'the v1 log is old and goes; other files stay');
    }

    public function testRunsAtMostOncePerInterval(): void
    {
        $rotation = new Rotation(1, 0, 0, 60);
        $old = $this->log(self::NOW - 2 * self::DAY);

        self::assertTrue($rotation->due($this->logs(), self::NOW));
        $rotation->run($this->logs(), self::NOW);
        $this->log(self::NOW - 3 * self::DAY);

        self::assertFalse($rotation->due($this->logs(), self::NOW + 30000));
        self::assertTrue($rotation->due($this->logs(), self::NOW + 61000));
        self::assertNotContains($old, $this->names());
    }

    public function testDebuggerRotatesAtStart(): void
    {
        $old = $this->log(self::NOW - 40 * self::DAY);
        $debugger = Debugger::start(
            Config::fromArray(['enabled' => true, 'logs_dir' => $this->logs(), 'max_age_days' => 30]),
            RequestInfo::cli([]),
            new FixedClock(self::NOW)
        );

        self::assertNotContains($old, $this->names());
        self::assertCount(1, $this->names(), 'only the file of this request');
        self::assertTrue($debugger->enabled());
    }

    public function testConfigDefaultsAndTheV1SizeLimit(): void
    {
        $config = Config::fromArray([]);
        self::assertSame([30, 5000, 256 * 1024 * 1024], [$config->maxAgeDays(), $config->maxFiles(), $config->maxLogsBytes()]);

        $v1 = Config::fromArray(['iLogsSizeLimit' => 2048]);
        self::assertSame(2048 * 1024, $v1->maxLogsBytes(), 'v1 counted kilobytes');
        self::assertSame(0, Config::fromArray(['max_files' => 0])->maxFiles(), '0 turns a limit off');
    }
}
