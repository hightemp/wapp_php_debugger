<?php

declare(strict_types=1);

namespace WappDebugger\Tests;

use PHPUnit\Framework\TestCase;
use WappDebugger\Config;
use WappDebugger\Debugger;
use WappDebugger\RequestInfo;
use WappDebugger\Tests\Support\FixedClock;
use WappDebugger\Tests\Support\LogAssert;
use WappDebugger\Tests\Support\TempDir;

/**
 * The global helpers of library v1 keep working on top of 2.x (plan 10.23).
 */
final class CompatTest extends TestCase
{
    use TempDir;

    private FixedClock $clock;

    protected function setUp(): void
    {
        $this->clock = new FixedClock(1792400000000);
        Debugger::setInstance(Debugger::start(
            Config::fromArray(['enabled' => true, 'logs_dir' => $this->tempDir() . '/logs', 'project_root' => dirname(__DIR__), 'trace_frames' => 0]),
            RequestInfo::cli([]),
            $this->clock
        ));
    }

    protected function tearDown(): void
    {
        Debugger::setInstance(null);
    }

    /**
     * @return list<array<string, mixed>> events after request_start
     */
    private function events(): array
    {
        return array_slice(LogAssert::validStream($this->logFiles($this->tempDir() . '/logs')[0]), 1);
    }

    public function testDebugMessage(): void
    {
        $line = __LINE__ + 1;
        __d(DWS_ERROR, 'Order failed', ['order' => 7]);

        $event = $this->events()[0];
        self::assertSame('error', $event['level']);
        self::assertSame('Order failed', $event['message']);
        self::assertSame(['order' => 7], $event['context']);
        self::assertSame(['file' => 'tests/CompatTest.php', 'line' => $line, 'function' => __FUNCTION__, 'class' => self::class, 'call_type' => '->'], $event['source']);
    }

    public function testDebugMessageWithV1Types(): void
    {
        __d(DWS_FUNC_DEBUG, 42, 'not an array');
        __d(DWS_WARNING, 'w');
        __d(DWS_INFO, 'i');
        __d(DWS_MESSAGE, 'm');

        $events = $this->events();
        self::assertSame(['message', 'warning', 'info', 'message'], array_column($events, 'level'));
        self::assertSame('42', $events[0]['message']);
        self::assertSame(['value' => 'not an array'], $events[0]['context']);
    }

    public function testInlineLogReturnsTheValue(): void
    {
        $line = __LINE__ + 1;
        $sum = __log(2) + __log(['id' => 7])['id'];

        self::assertSame(9, $sum);
        $events = $this->events();
        self::assertSame('2', $events[0]['message']);
        self::assertSame(['inlinelog', 'integer'], $events[0]['tags']);
        self::assertSame(['value' => 2], $events[0]['context']);
        self::assertSame('7', $events[1]['message'], 'a one-item array shows its value, as in v1');
        self::assertSame(['id' => 7], $events[1]['context']);
        self::assertSame($line, $events[1]['source']['line']);
    }

    public function testNamedTimersGetALevelByDuration(): void
    {
        foreach (['fast' => 5, 'slow' => 1500, 'very-slow' => 2500] as $name => $duration) {
            __start($name);
            $this->clock->advance($duration);
            __stop($name);
        }

        $events = $this->events();
        self::assertSame(['fast', 'slow', 'very-slow'], array_column($events, 'name'));
        self::assertSame(['info', 'warning', 'error'], array_column($events, 'level'));
    }

    public function testUnnamedTimersAreAStack(): void
    {
        __start();
        $this->clock->advance(10);
        __start();
        $this->clock->advance(5);
        __stop();
        __stop();

        $events = $this->events();
        self::assertSame(['2', '1'], array_column($events, 'name'));
        self::assertSame([5.0, 15.0], array_map('floatval', array_column($events, 'duration_ms')));
    }

    public function testDeprecatedFunctionsWarnOncePerRequest(): void
    {
        $deprecations = [];
        set_error_handler(static function (int $type, string $message) use (&$deprecations): bool {
            $deprecations[] = $message;

            return true;
        }, E_USER_DEPRECATED);
        try {
            fnDebugLog(DWS_MESSAGE, 'one');
            fnDebugLog(DWS_MESSAGE, 'two');
            fnDebugAddFoundField('email');
        } finally {
            restore_error_handler();
        }

        self::assertCount(2, $deprecations, 'once per function');
        self::assertStringContainsString('fnDebugLog() is deprecated, use __d()', $deprecations[0]);
        self::assertSame(['one', 'two'], array_slice(array_column($this->events(), 'message'), 0, 2));
    }

    public function testHelpersDoNothingWithoutADebugger(): void
    {
        Debugger::setInstance(null);

        __d(DWS_ERROR, 'nowhere');
        __start('t');
        __stop('t');

        self::assertSame('kept', __log('kept'));
        self::assertSame([], $this->logFiles($this->tempDir() . '/logs') === [] ? [] : $this->events());
    }
}
