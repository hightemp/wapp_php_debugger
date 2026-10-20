<?php

declare(strict_types=1);

namespace WappDebugger\Tests;

use PHPUnit\Framework\TestCase;
use WappDebugger\Bridge\GuzzleMiddleware;
use WappDebugger\Config;
use WappDebugger\Debugger;
use WappDebugger\RequestInfo;
use WappDebugger\Sql;
use WappDebugger\Tests\Support\FixedClock;
use WappDebugger\Tests\Support\LogAssert;
use WappDebugger\Tests\Support\TempDir;

/**
 * SQL queries and outgoing HTTP calls: messages with a tag and a duration, which the viewer lists
 * on the SQL tab and the timeline.
 */
final class TimedCallsTest extends TestCase
{
    use TempDir;

    private function debugger(?FixedClock $clock = null): Debugger
    {
        $config = Config::fromArray(['enabled' => true, 'logs_dir' => $this->tempDir() . '/logs', 'project_root' => dirname(__DIR__)]);
        $debugger = Debugger::start($config, RequestInfo::web('GET', 'https://shop.test/orders'), $clock ?? new FixedClock(1792400000000));
        Debugger::setInstance($debugger);

        return $debugger;
    }

    /**
     * @return list<array<string, mixed>>
     */
    private function messages(): array
    {
        $events = LogAssert::validStream($this->logFiles($this->tempDir() . '/logs')[0]);

        return array_values(array_filter($events, static function (array $event): bool {
            return $event['type'] === 'message';
        }));
    }

    protected function tearDown(): void
    {
        Debugger::setInstance(null);
    }

    public function testAnSqlQueryWithItsDurationAndMaskedParameters(): void
    {
        $this->debugger()->sql('SELECT * FROM users WHERE email = :email AND password = :password', [':email' => 'a@b.c', ':password' => 'secret'], 4.25, 1);

        $message = $this->messages()[0];
        self::assertSame(['sql'], $message['tags']);
        self::assertSame('debug', $message['level']);
        self::assertSame('SELECT * FROM users WHERE email = :email AND password = :password', $message['message']);
        self::assertSame(4.25, $message['duration_ms']);
        self::assertSame(['params' => [':email' => 'a@b.c', ':password' => '***'], 'rows' => 1], $message['context']);
        self::assertSame('tests/TimedCallsTest.php', $message['source']['file']);
    }

    public function testAnHttpCallWithItsStatusAndNoSecretsInTheUrl(): void
    {
        $debugger = $this->debugger();
        $debugger->http('get', 'https://pay.test/status?id=7&token=abc&api_key=k', 200, 120.5);
        $debugger->http('POST', 'https://pay.test/charge', 502, 30.0);
        $debugger->http('POST', 'https://pay.test/charge', null, 5000.0, 'Connection timed out');

        [$ok, $bad, $failed] = $this->messages();
        self::assertSame(['http'], $ok['tags']);
        self::assertSame('GET https://pay.test/status?id=7&token=***&api_key=***', $ok['message']);
        self::assertSame('info', $ok['level']);
        self::assertSame(['status' => 200], $ok['context']);
        self::assertSame(120.5, $ok['duration_ms']);
        self::assertSame('warning', $bad['level']);
        self::assertSame('warning', $failed['level']);
        self::assertSame(['error' => 'Connection timed out'], $failed['context']);
    }

    public function testPreparedStatementsOfAWatchedPdoAreTimed(): void
    {
        if (!in_array('sqlite', \PDO::getAvailableDrivers(), true)) {
            self::markTestSkipped('pdo_sqlite is not available');
        }
        $clock = new FixedClock(1792400000000);
        $this->debugger($clock);
        $pdo = new \PDO('sqlite::memory:', null, null, [\PDO::ATTR_ERRMODE => \PDO::ERRMODE_EXCEPTION]);
        $pdo->exec('CREATE TABLE items (id INTEGER PRIMARY KEY, name TEXT)');
        Sql::watch($pdo);

        $insert = $pdo->prepare('INSERT INTO items (id, name) VALUES (?, ?)');
        $insert->execute([1, 'pen']);
        $select = $pdo->prepare('SELECT * FROM items WHERE id = :id');
        $select->bindValue(':id', 1);
        $select->execute();
        // PHP 7.4 gives SQLite integers as strings: only the name is compared
        $rows = $select->fetchAll(\PDO::FETCH_ASSOC);
        self::assertIsArray($rows);
        self::assertSame(['pen'], array_column($rows, 'name'), 'the statement still works');
        try {
            $insert->execute([1, 'again']); // the same primary key
            self::fail('no exception');
        } catch (\PDOException $exception) {
            self::assertStringContainsString('UNIQUE', $exception->getMessage());
        }

        [$inserted, $selected, $failed] = $this->messages();
        self::assertSame('INSERT INTO items (id, name) VALUES (?, ?)', $inserted['message']);
        self::assertSame(['params' => [1, 'pen'], 'rows' => 1], $inserted['context']);
        self::assertArrayHasKey('duration_ms', $inserted);
        self::assertSame(['params' => [':id' => 1]], array_intersect_key($selected['context'], ['params' => 0]));
        self::assertSame('tests/TimedCallsTest.php', $selected['source']['file'], 'the call site is the application, not the wrapper');
        self::assertSame('error', $failed['level']);
        self::assertSame([1, 'again'], $failed['context']['params']);
        self::assertStringContainsString('UNIQUE', $failed['context']['error']);
    }

    public function testGuzzleRequestsThroughTheMiddleware(): void
    {
        $this->debugger();
        $request = new class () {
            public function getMethod(): string
            {
                return 'GET';
            }

            public function getUri(): string
            {
                return 'https://api.test/users?token=t';
            }
        };
        $response = new class () {
            public function getStatusCode(): int
            {
                return 404;
            }
        };
        $handler = static function ($request, array $options) use ($response) {
            return new FakePromise($response, null);
        };
        $failing = static function ($request, array $options) {
            return new FakePromise(null, new \RuntimeException('cURL error 28: timed out'));
        };

        $stack = GuzzleMiddleware::create();
        $result = $stack($handler)($request, []);
        self::assertSame($response, $result->value, 'the response passes through');
        $result = $stack($failing)($request, []);
        self::assertInstanceOf(\RuntimeException::class, $result->error, 'the failure passes through');

        [$notFound, $timedOut] = $this->messages();
        self::assertSame('GET https://api.test/users?token=***', $notFound['message']);
        self::assertSame(['status' => 404], $notFound['context']);
        self::assertSame('warning', $notFound['level']);
        self::assertSame(['error' => 'cURL error 28: timed out'], $timedOut['context']);
    }
}

/**
 * Just enough of a Guzzle promise: then() runs a callback at once.
 */
final class FakePromise
{
    /** @var mixed */
    public $value;
    public ?\Throwable $error;

    /**
     * @param mixed $value
     */
    public function __construct($value, ?\Throwable $error)
    {
        $this->value = $value;
        $this->error = $error;
    }

    public function then(?callable $fulfilled = null, ?callable $rejected = null): self
    {
        try {
            if ($this->error === null) {
                return new self($fulfilled !== null ? $fulfilled($this->value) : $this->value, null);
            }
            if ($rejected === null) {
                return $this;
            }

            return new self($rejected($this->error), null);
        } catch (\Throwable $error) {
            return new self(null, $error);
        }
    }
}
