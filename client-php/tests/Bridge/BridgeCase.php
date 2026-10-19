<?php

declare(strict_types=1);

namespace WappDebugger\Tests\Bridge;

use PHPUnit\Framework\TestCase;
use WappDebugger\Config;
use WappDebugger\Debugger;
use WappDebugger\RequestInfo;
use WappDebugger\Tests\Support\FixedClock;
use WappDebugger\Tests\Support\LogAssert;
use WappDebugger\Tests\Support\TempDir;

abstract class BridgeCase extends TestCase
{
    use TempDir;

    protected function debugger(): Debugger
    {
        return Debugger::start(
            Config::fromArray(['enabled' => true, 'logs_dir' => $this->tempDir() . '/logs', 'project_root' => dirname(__DIR__, 2), 'trace_frames' => 0]),
            RequestInfo::cli([]),
            new FixedClock(1792400000000)
        );
    }

    /**
     * @return list<array<string, mixed>> events after request_start
     */
    protected function events(): array
    {
        return array_slice(LogAssert::validStream($this->logFiles($this->tempDir() . '/logs')[0]), 1);
    }
}
