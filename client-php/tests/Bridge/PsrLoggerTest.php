<?php

declare(strict_types=1);

namespace WappDebugger\Tests\Bridge;

use Psr\Log\InvalidArgumentException;
use Psr\Log\LoggerInterface;
use Psr\Log\LogLevel;
use WappDebugger\Bridge\PsrLogger;

final class PsrLoggerTest extends BridgeCase
{
    public function testIsAPsr3Logger(): void
    {
        self::assertInstanceOf(LoggerInterface::class, new PsrLogger($this->debugger()));
    }

    public function testMapsTheEightLevels(): void
    {
        $logger = new PsrLogger($this->debugger());
        foreach ([LogLevel::EMERGENCY, LogLevel::ALERT, LogLevel::CRITICAL, LogLevel::ERROR, LogLevel::WARNING, LogLevel::NOTICE, LogLevel::INFO, LogLevel::DEBUG] as $level) {
            $logger->log($level, $level);
        }

        self::assertSame(['error', 'error', 'error', 'error', 'warning', 'info', 'info', 'debug'], array_column($this->events(), 'level'));
    }

    public function testInterpolatesPlaceholdersAndKeepsTheContext(): void
    {
        $line = __LINE__ + 1;
        (new PsrLogger($this->debugger()))->info('User {user} paid {sum} at {when}', ['user' => 'ann', 'sum' => 9.5, 'when' => new \DateTimeImmutable('@0'), 'token' => 't']);

        $event = $this->events()[0];
        self::assertSame('User ann paid 9.5 at 1970-01-01T00:00:00+00:00', $event['message']);
        self::assertSame('***', $event['context']['token']);
        self::assertSame(['psr3'], $event['tags']);
        self::assertSame(['file' => 'tests/Bridge/PsrLoggerTest.php', 'line' => $line, 'function' => __FUNCTION__, 'class' => self::class, 'call_type' => '->'], $event['source'], 'the call site, not the PSR-3 classes');
    }

    public function testAnExceptionInTheContextIsWrittenWithItsTrace(): void
    {
        (new PsrLogger($this->debugger()))->error('Payment failed', ['exception' => new \RuntimeException('gateway timeout')]);

        $events = $this->events();
        self::assertSame(['message', 'error'], array_column($events, 'type'));
        self::assertSame('Payment failed', $events[0]['message']);
        self::assertSame('gateway timeout', $events[1]['message']);
        self::assertSame(\RuntimeException::class, $events[1]['exception_class']);
    }

    public function testRejectsUnknownLevelsAsPsr3Requires(): void
    {
        $this->expectException(InvalidArgumentException::class);

        (new PsrLogger($this->debugger()))->log('loud', 'x');
    }
}
