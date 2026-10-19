<?php

declare(strict_types=1);

namespace WappDebugger\Tests\Bridge;

use Monolog\Logger;
use WappDebugger\Bridge\MonologHandler;

final class MonologHandlerTest extends BridgeCase
{
    public function testWritesRecordsOfTheInstalledMonolog(): void
    {
        $logger = new Logger('billing', [new MonologHandler($this->debugger())]);
        $logger->pushProcessor(static function ($record) {
            // Monolog 2 passes an array, Monolog 3 a LogRecord with a mutable $extra
            if (is_array($record)) {
                $record['extra']['host'] = 'web-1';
            } else {
                $record->extra['host'] = 'web-1';
            }

            return $record;
        });

        $line = __LINE__ + 1;
        $logger->warning('Invoice {id} is late', ['id' => 42, 'password' => 'p']);
        $logger->debug('details');
        $logger->critical('down', ['exception' => new \LogicException('db is gone')]);

        $events = $this->events();
        self::assertSame(['message', 'message', 'message', 'error'], array_column($events, 'type'));
        self::assertSame(['warning', 'debug', 'error'], array_column(array_slice($events, 0, 3), 'level'));
        self::assertSame('Invoice {id} is late', $events[0]['message'], 'Monolog interpolates only with PsrLogMessageProcessor');
        self::assertSame(['id' => 42, 'password' => '***', 'extra' => ['host' => 'web-1']], $events[0]['context']);
        self::assertSame(['monolog', 'billing'], $events[0]['tags']);
        self::assertSame('tests/Bridge/MonologHandlerTest.php', $events[0]['source']['file'], 'the call site, not Monolog');
        self::assertSame($line, $events[0]['source']['line']);
        self::assertSame('db is gone', $events[3]['message']);
    }

    public function testRespectsTheHandlerLevel(): void
    {
        $logger = new Logger('app', [new MonologHandler($this->debugger(), Logger::WARNING)]);
        $logger->info('skipped');
        $logger->error('kept');

        self::assertSame(['kept'], array_column($this->events(), 'message'));
    }
}
