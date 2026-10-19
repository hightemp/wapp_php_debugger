<?php

declare(strict_types=1);

namespace WappDebugger\Tests;

use PHPUnit\Framework\TestCase;
use WappDebugger\Config;
use WappDebugger\Debugger;
use WappDebugger\ErrorHandler;
use WappDebugger\RequestInfo;
use WappDebugger\Tests\Support\FixedClock;
use WappDebugger\Tests\Support\LogAssert;
use WappDebugger\Tests\Support\TempDir;

final class ErrorHandlerTest extends TestCase
{
    use TempDir;

    private function handler(?RequestInfo $request = null, ?int $responseCode = null): ErrorHandler
    {
        $debugger = Debugger::start(
            Config::fromArray(['enabled' => true, 'logs_dir' => $this->tempDir() . '/logs', 'project_root' => dirname(__DIR__)]),
            $request ?? RequestInfo::cli([]),
            new FixedClock(1792400000000)
        );

        return new ErrorHandler($debugger, static function () use ($responseCode) {
            return $responseCode;
        });
    }

    /**
     * @return list<array<string, mixed>>
     */
    private function events(): array
    {
        return LogAssert::validStream($this->logFiles($this->tempDir() . '/logs')[0]);
    }

    public function testPhpWarning(): void
    {
        $handled = $this->handler()->handleError(E_WARNING, 'Undefined index: discount', __FILE__, 42);

        self::assertFalse($handled, 'without a previous handler PHP reports the error as usual');
        $error = $this->events()[1];
        self::assertSame('error', $error['type']);
        self::assertSame('php_error', $error['kind']);
        self::assertSame('warning', $error['level']);
        self::assertSame('E_WARNING', $error['error_type']);
        self::assertSame('Undefined index: discount', $error['message']);
        self::assertSame(['file' => 'tests/ErrorHandlerTest.php', 'line' => 42], $error['source']);
    }

    public function testLevelsOfPhpErrors(): void
    {
        $handler = $this->handler();
        $old = error_reporting(E_ALL);
        try {
            foreach ([E_NOTICE, E_DEPRECATED, E_USER_DEPRECATED, E_USER_WARNING, E_USER_ERROR, E_RECOVERABLE_ERROR] as $type) {
                $handler->handleError($type, 'x', __FILE__, 1);
            }
        } finally {
            error_reporting($old);
        }

        $events = array_slice($this->events(), 1);
        self::assertSame(
            ['E_NOTICE', 'E_DEPRECATED', 'E_USER_DEPRECATED', 'E_USER_WARNING', 'E_USER_ERROR', 'E_RECOVERABLE_ERROR'],
            array_column($events, 'error_type')
        );
        self::assertSame(['info', 'info', 'info', 'warning', 'error', 'error'], array_column($events, 'level'));
    }

    public function testErrorsOutsideErrorReportingAreSkipped(): void
    {
        $old = error_reporting(E_ALL & ~E_NOTICE);
        try {
            $this->handler()->handleError(E_NOTICE, 'silenced', __FILE__, 1);
        } finally {
            error_reporting($old);
        }

        self::assertCount(1, $this->events());
    }

    public function testPreviousHandlerIsCalled(): void
    {
        $calls = [];
        $handler = $this->handler();
        $handler->chainErrorHandler(static function (int $type, string $message) use (&$calls): bool {
            $calls[] = [$type, $message];

            return true;
        });

        self::assertTrue($handler->handleError(E_WARNING, 'w', __FILE__, 1));
        self::assertSame([[E_WARNING, 'w']], $calls);
    }

    public function testException(): void
    {
        $previous = new \InvalidArgumentException('bad id', 3);
        $line = __LINE__ + 1;
        $exception = new \RuntimeException('Order not found', 404, $previous);

        $this->handler()->logException($exception, true);

        $error = $this->events()[1];
        self::assertSame('exception', $error['kind']);
        self::assertSame('error', $error['level']);
        self::assertSame(\RuntimeException::class, $error['exception_class']);
        self::assertSame('Order not found', $error['message']);
        self::assertSame(404, $error['code']);
        self::assertSame(['file' => 'tests/ErrorHandlerTest.php', 'line' => $line], $error['source']);
        self::assertNotEmpty($error['trace'], 'errors keep the full trace');
        self::assertSame(\InvalidArgumentException::class, $error['previous'][0]['exception_class']);
        self::assertSame('bad id', $error['previous'][0]['message']);
        self::assertSame(3, $error['previous'][0]['code']);
    }

    public function testShutdownAfterAFatalError(): void
    {
        $this->handler(RequestInfo::web('GET', '/'), 200)->handleShutdown(['type' => E_ERROR, 'message' => 'Allowed memory size exhausted', 'file' => __FILE__, 'line' => 7]);

        $events = $this->events();
        self::assertSame(['request_start', 'error', 'request_end'], array_column($events, 'type'));
        self::assertSame('fatal', $events[1]['kind']);
        self::assertSame('E_ERROR', $events[1]['error_type']);
        self::assertSame(500, $events[2]['status_code'], 'PHP answers 500 after a fatal error');
    }

    public function testMemoryLimitIsNeverLowered(): void
    {
        $before = ini_get('memory_limit');
        $this->handler()->handleShutdown(['type' => E_ERROR, 'message' => 'Allowed memory size of 1 bytes exhausted', 'file' => __FILE__, 'line' => 1]);

        self::assertSame($before, ini_get('memory_limit'));
    }

    public function testShutdownOfANormalRequest(): void
    {
        $this->handler(RequestInfo::web('GET', '/'), 201)->handleShutdown(null);

        $events = $this->events();
        self::assertSame(['request_start', 'request_end'], array_column($events, 'type'));
        self::assertSame(201, $events[1]['status_code']);
    }

    public function testShutdownOfACliProcessAfterAnUncaughtException(): void
    {
        $handler = $this->handler();
        $handler->logException(new \LogicException('x'), true);
        $handler->handleShutdown(null);

        self::assertSame(255, $this->events()[2]['exit_code']);
    }

    public function testShutdownIgnoresAnOldNonFatalError(): void
    {
        $this->handler()->handleShutdown(['type' => E_WARNING, 'message' => 'handled earlier', 'file' => __FILE__, 'line' => 1]);

        self::assertSame(['request_start', 'request_end'], array_column($this->events(), 'type'));
    }
}
