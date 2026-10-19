<?php

declare(strict_types=1);

namespace WappDebugger\Tests;

use PHPUnit\Framework\TestCase;
use WappDebugger\Config;
use WappDebugger\Tests\Support\TempDir;

final class ConfigTest extends TestCase
{
    use TempDir;

    public function testDefaults(): void
    {
        $config = Config::fromArray([]);

        self::assertFalse($config->enabled());
        self::assertSame(0664, $config->fileMode());
        self::assertSame(0775, $config->dirMode());
    }

    public function testMissingFileMeansDisabled(): void
    {
        self::assertFalse(Config::fromFile($this->tempDir() . '/dws_debugger.json')->enabled());
    }

    public function testReadsTheFileAndResolvesPathsAgainstIt(): void
    {
        $file = $this->tempDir() . '/dws_debugger.json';
        file_put_contents($file, '{"enabled": true, "logs_dir": "tmp/logs"}');

        $config = Config::fromFile($file);

        self::assertTrue($config->enabled());
        self::assertSame($this->tempDir(), $config->projectRoot(), 'the root defaults to the directory of the config');
        self::assertSame($this->tempDir() . '/tmp/logs', $config->logsDir());
    }

    public function testBrokenFileMeansDisabled(): void
    {
        $file = $this->tempDir() . '/dws_debugger.json';
        file_put_contents($file, '{"enabled": tru');

        self::assertFalse(Config::fromFile($file)->enabled());
    }
}
