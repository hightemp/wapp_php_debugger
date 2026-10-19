<?php

declare(strict_types=1);

namespace WappDebugger\Tests\Support;

use Opis\JsonSchema\Errors\ErrorFormatter;
use Opis\JsonSchema\Validator;
use PHPUnit\Framework\Assert;

/**
 * Checks a v2 log file against the same contract the viewer relies on:
 * every line matches spec/logformat/v2/event.schema.json and the stream rules hold.
 */
final class LogAssert
{
    private static ?Validator $validator = null;

    /**
     * @return list<array<string, mixed>> the decoded events
     */
    public static function validStream(string $file): array
    {
        Assert::assertFileExists($file);
        $content = (string) file_get_contents($file);
        Assert::assertStringEndsWith("\n", $content, 'every event ends with a newline');

        $events = [];
        foreach (explode("\n", rtrim($content, "\n")) as $number => $line) {
            $data = json_decode($line, false, 512, JSON_THROW_ON_ERROR);
            $result = self::validator()->validate($data, 'https://github.com/hightemp/wapp_php_debugger/spec/logformat/v2/event.schema.json');
            Assert::assertTrue(
                $result->isValid(),
                sprintf("line %d does not match the v2 schema: %s\n%s", $number + 1, json_encode(
                    $result->error() !== null ? (new ErrorFormatter())->format($result->error()) : [],
                    JSON_UNESCAPED_UNICODE
                ), $line)
            );
            /** @var array<string, mixed> $event */
            $event = json_decode($line, true, 512, JSON_THROW_ON_ERROR);
            $events[] = $event;
        }

        Assert::assertSame('request_start', $events[0]['type'], 'the first event is request_start');
        $previousTs = 0;
        foreach ($events as $index => $event) {
            Assert::assertSame($events[0]['request_id'], $event['request_id'], 'one request_id per file');
            Assert::assertSame($index + 1, $event['seq'], 'seq starts at 1 and has no gaps');
            Assert::assertGreaterThanOrEqual($previousTs, $event['ts'], 'ts never decreases');
            $previousTs = $event['ts'];
            if ($index > 0) {
                Assert::assertNotSame('request_start', $event['type'], 'only one request_start');
            }
            if ($event['type'] === 'request_end') {
                Assert::assertSame(count($events) - 1, $index, 'request_end is the last event');
            }
        }

        return $events;
    }

    private static function validator(): Validator
    {
        if (self::$validator === null) {
            self::$validator = new Validator();
            $resolver = self::$validator->resolver();
            assert($resolver !== null);
            $resolver->registerPrefix(
                'https://github.com/hightemp/wapp_php_debugger/spec/',
                dirname(__DIR__, 3) . '/spec/'
            );
        }

        return self::$validator;
    }
}
