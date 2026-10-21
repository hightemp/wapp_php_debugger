<?php

declare(strict_types=1);

// A Bitrix request as /local/php_interface/init.php sees it: events of the main module and the
// tracker of the D7 connection

namespace Bitrix\Main {
    final class TrackedQuery
    {
        public function getSql(): string
        {
            return 'SELECT * FROM b_iblock_element WHERE ID = 7';
        }

        public function getTime(): float
        {
            return 0.0042;
        }
    }

    final class Tracker
    {
        /** @return list<TrackedQuery> */
        public function getQueries(): array
        {
            return [new TrackedQuery()];
        }
    }

    final class Connection
    {
        public function startTracker(): Tracker
        {
            return new Tracker();
        }
    }

    final class Application
    {
        public static function getConnection(): Connection
        {
            return new Connection();
        }
    }
}

namespace {
    $GLOBALS['wapp_test_events'] = [];

    function AddEventHandler(string $module, string $event, callable $callback): int
    {
        $GLOBALS['wapp_test_events'][$module . ':' . $event][] = $callback;

        return 1;
    }

    $_SERVER['DOCUMENT_ROOT'] = $argv[1];
    require dirname(__DIR__, 3) . '/vendor/autoload.php';
    require dirname(__DIR__, 3) . '/integrations/bitrix/wapp-debugger.php';

    $content = '<html><body><h1>Catalog</h1></body></html>';
    foreach ($GLOBALS['wapp_test_events']['main:OnEndBufferContent'] ?? [] as $callback) {
        $callback($content);
    }
    echo $content;
    foreach ($GLOBALS['wapp_test_events']['main:OnAfterEpilog'] ?? [] as $callback) {
        $callback();
    }
}
