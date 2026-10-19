<?php

declare(strict_types=1);

namespace WappDebugger\Tests;

use PHPUnit\Framework\TestCase;
use WappDebugger\Config;
use WappDebugger\Debugger;
use WappDebugger\Level;
use WappDebugger\RequestInfo;
use WappDebugger\Tests\Support\FixedClock;
use WappDebugger\Tests\Support\LogAssert;
use WappDebugger\Tests\Support\TempDir;

final class DebuggerDetailsTest extends TestCase
{
    use TempDir;

    private const START = 1792400000000;

    /**
     * @param array<string, mixed> $config
     */
    private function debugger(FixedClock $clock, array $config = []): Debugger
    {
        // The project root is this test directory, so paths in source and trace become relative to it
        return Debugger::start(
            Config::fromArray($config + ['enabled' => true, 'logs_dir' => $this->tempDir() . '/logs', 'project_root' => dirname(__DIR__)]),
            RequestInfo::cli([]),
            $clock
        );
    }

    /**
     * @return list<array<string, mixed>>
     */
    private function events(): array
    {
        return LogAssert::validStream($this->logFiles($this->tempDir() . '/logs')[0]);
    }

    public function testSourceIsTheCallSiteRelativeToTheProjectRoot(): void
    {
        $debugger = $this->debugger(new FixedClock(self::START));
        $line = __LINE__ + 1;
        $debugger->log(Level::INFO, 'here');

        $source = $this->events()[1]['source'];
        self::assertSame('tests/DebuggerDetailsTest.php', $source['file']);
        self::assertSame($line, $source['line']);
        self::assertSame(__FUNCTION__, $source['function']);
        self::assertSame(self::class, $source['class']);
        self::assertSame('->', $source['call_type']);
    }

    public function testTraceIsShortByDefault(): void
    {
        $debugger = $this->debugger(new FixedClock(self::START), ['trace_frames' => 2]);
        $this->nested($debugger, 5);

        $trace = $this->events()[1]['trace'];
        self::assertCount(2, $trace);
        self::assertSame('nested', $trace[0]['function'], 'the innermost call first');
        self::assertSame('tests/DebuggerDetailsTest.php', $trace[0]['file']);
    }

    public function testTraceCanBeTurnedOff(): void
    {
        $debugger = $this->debugger(new FixedClock(self::START), ['trace_frames' => 0]);
        $debugger->log(Level::INFO, 'no trace');

        self::assertArrayNotHasKey('trace', $this->events()[1]);
        self::assertArrayHasKey('source', $this->events()[1]);
    }

    public function testPathsOutsideTheRootStayAbsolute(): void
    {
        $debugger = $this->debugger(new FixedClock(self::START), ['project_root' => '/somewhere/else']);
        $debugger->log(Level::INFO, 'here');

        self::assertSame(__FILE__, $this->events()[1]['source']['file']);
    }

    public function testTagsAreNormalized(): void
    {
        $debugger = $this->debugger(new FixedClock(self::START));
        $debugger->log(Level::DEBUG, 'SELECT 1', [], ['#SQL', 'sql', 'slow query', 'db.read', '']);

        self::assertSame(['sql', 'db.read'], $this->events()[1]['tags'], 'without #, lower case, unique; invalid tags are dropped');
    }

    public function testTimer(): void
    {
        $clock = new FixedClock(self::START);
        $debugger = $this->debugger($clock);
        $debugger->startTimer('db');
        $clock->advance(12);
        $line = __LINE__ + 1;
        $debugger->stopTimer('db');

        $timer = $this->events()[1];
        self::assertSame('timer', $timer['type']);
        self::assertSame('db', $timer['name']);
        self::assertSame(12.0, (float) $timer['duration_ms']);
        self::assertSame('info', $timer['level'], 'a fast timer');
        self::assertSame($line, $timer['source']['line']);
    }

    public function testStoppingAnUnknownTimerIsAWarning(): void
    {
        $debugger = $this->debugger(new FixedClock(self::START));
        $debugger->stopTimer('never-started');

        $event = $this->events()[1];
        self::assertSame('message', $event['type']);
        self::assertSame('warning', $event['level']);
        self::assertStringContainsString('never-started', $event['message']);
    }

    public function testParentFromTheHeaderOfAWebRequest(): void
    {
        $parent = '01JAZ7Q5X0K4M8T2V6N9P3R5S7';
        $info = RequestInfo::fromGlobals(['REQUEST_METHOD' => 'POST', 'HTTP_HOST' => 'shop.test', 'REQUEST_URI' => '/api/cart',
            'HTTP_X_WAPP_DEBUGGER_PARENT' => $parent, 'HTTP_X_REQUESTED_WITH' => 'XMLHttpRequest'], 'fpm-fcgi', []);

        self::assertSame($parent, $info->fields()['parent_request_id']);
        self::assertTrue($info->fields()['is_ajax']);
    }

    public function testParentFromTheEnvironmentOfAChildProcess(): void
    {
        $info = RequestInfo::fromGlobals(['argv' => ['worker.php']], 'cli', ['WAPP_DEBUGGER_PARENT' => '01JAZ7Q5X0K4M8T2V6N9P3R5S7']);

        self::assertSame('01JAZ7Q5X0K4M8T2V6N9P3R5S7', $info->fields()['parent_request_id']);
    }

    public function testAMalformedParentIsIgnored(): void
    {
        $info = RequestInfo::fromGlobals(['argv' => []], 'cli', ['WAPP_DEBUGGER_PARENT' => 'not-a-ulid']);

        self::assertArrayNotHasKey('parent_request_id', $info->fields());
    }

    public function testChildProcessesInheritTheRequestId(): void
    {
        $before = getenv('WAPP_DEBUGGER_PARENT');
        try {
            $debugger = $this->debugger(new FixedClock(self::START));

            self::assertSame($debugger->requestId(), getenv('WAPP_DEBUGGER_PARENT'));
            self::assertSame($debugger->requestId(), trim((string) shell_exec('echo $WAPP_DEBUGGER_PARENT')));
        } finally {
            putenv($before === false ? 'WAPP_DEBUGGER_PARENT' : 'WAPP_DEBUGGER_PARENT=' . $before);
        }
    }

    private function nested(Debugger $debugger, int $levels): void
    {
        if ($levels === 0) {
            $debugger->log(Level::INFO, 'deep');

            return;
        }
        $this->nested($debugger, $levels - 1);
    }
}
