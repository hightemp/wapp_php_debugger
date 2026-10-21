<?php

declare(strict_types=1);

namespace WappDebugger\Tests\Bridge;

use Monolog\Logger;
use WappDebugger\Bridge\LaravelChannel;
use WappDebugger\Debugger;

final class LaravelChannelTest extends BridgeCase
{
    protected function tearDown(): void
    {
        Debugger::setInstance(null);
    }

    public function testIsACustomChannelOfConfigLogging(): void
    {
        Debugger::setInstance($this->debugger());
        // config/logging.php: 'wapp-debugger' => ['driver' => 'custom', 'via' => LaravelChannel::class, 'level' => 'info']
        $logger = (new LaravelChannel())(['driver' => 'custom', 'via' => LaravelChannel::class, 'level' => 'info']);

        self::assertInstanceOf(Logger::class, $logger);
        self::assertSame('wapp-debugger', $logger->getName());
        $logger->debug('below the level of the channel');
        $logger->error('Payment failed', ['order' => 7]);

        $events = $this->events();
        self::assertCount(1, $events);
        self::assertSame('error', $events[0]['level']);
        self::assertSame(['order' => 7], $events[0]['context']);
    }

    public function testTakesTheNameOfTheChannelAndLogsEverythingByDefault(): void
    {
        Debugger::setInstance($this->debugger());
        $logger = (new LaravelChannel())(['name' => 'stack']);
        $logger->debug('everything');

        self::assertSame('stack', $logger->getName());
        self::assertCount(1, $this->events());
    }
}
