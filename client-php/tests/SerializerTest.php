<?php

declare(strict_types=1);

namespace WappDebugger\Tests;

use PHPUnit\Framework\TestCase;
use WappDebugger\Masker;
use WappDebugger\Serializer;

final class SerializerTest extends TestCase
{
    private function serializer(int $maxDepth = 6, int $maxItems = 100, int $maxStringBytes = 2000): Serializer
    {
        return new Serializer(new Masker(), $maxDepth, $maxItems, $maxStringBytes);
    }

    public function testScalarsStayAsTheyAre(): void
    {
        $serializer = $this->serializer();

        self::assertSame([1, 1.5, true, null, 'text'], $serializer->normalize([1, 1.5, true, null, 'text']));
    }

    public function testFloatsThatJsonCannotHold(): void
    {
        self::assertSame(['NAN', 'INF', '-INF'], $this->serializer()->normalize([NAN, INF, -INF]));
    }

    public function testMasksSecretsAtAnyDepth(): void
    {
        $value = ['user' => ['name' => 'Тест', 'password' => 'p', 'tokens' => ['a', 'b']], 'Authorization' => 'Bearer x'];

        self::assertSame(
            ['user' => ['name' => 'Тест', 'password' => '***', 'tokens' => '***'], 'Authorization' => '***'],
            $this->serializer()->normalize($value)
        );
    }

    public function testCutsLongStringsOnACharacterBoundary(): void
    {
        $value = $this->serializer(6, 100, 10)->normalize(str_repeat('ж', 20)); // 2 bytes each

        self::assertSame(str_repeat('ж', 5) . '… (+30 bytes)', $value);
        self::assertNotFalse(json_encode($value));
    }

    public function testInvalidUtf8DoesNotBreakTheEvent(): void
    {
        self::assertNotFalse(json_encode($this->serializer()->normalize(["ok\xB1\x31"])));
    }

    public function testLimitsItems(): void
    {
        $value = $this->serializer(6, 3)->normalize(range(1, 10));

        self::assertSame([1, 2, 3, '…' => '+7 more'], $value);
    }

    public function testLimitsDepth(): void
    {
        $value = $this->serializer(2)->normalize(['a' => ['b' => ['c' => ['d' => 1]]]]);

        self::assertSame(['a' => ['b' => '[array(1)]']], $value);
    }

    public function testObjectsShowTheirClassAndAllProperties(): void
    {
        $value = $this->serializer()->normalize(new SampleUser('ann', 's3cret'));

        self::assertIsArray($value);
        self::assertSame('__class', array_key_first($value), 'the class comes first');
        ksort($value);
        // Private and protected properties too, including private ones of the parent class
        self::assertSame(['__class' => SampleUser::class, 'name' => 'ann', 'parentNote' => 'base', 'password' => '***', 'role' => 'admin'], $value);
    }

    public function testRecursionIsCut(): void
    {
        $node = new \stdClass();
        $node->self = $node;
        $list = [1];
        $list[] = &$list;

        self::assertSame(['__class' => 'stdClass', 'self' => '*RECURSION stdClass*'], $this->serializer()->normalize($node));
        self::assertNotFalse(json_encode($this->serializer()->normalize($list)));
    }

    public function testTheSameObjectTwiceIsNotRecursion(): void
    {
        $shared = new \stdClass();
        $shared->id = 7;

        $value = $this->serializer()->normalize(['first' => $shared, 'second' => $shared]);

        self::assertSame(['__class' => 'stdClass', 'id' => 7], $value['second']);
    }

    public function testSpecialValues(): void
    {
        $stream = fopen('php://memory', 'rb');
        $closure = static function (): void {
        };
        $value = $this->serializer()->normalize([
            'stream' => $stream,
            'closure' => $closure,
            'date' => new \DateTimeImmutable('2026-10-19T12:00:00+03:00'),
            'error' => new \RuntimeException('boom', 5),
        ]);
        if (is_resource($stream)) {
            fclose($stream);
        }

        self::assertSame('resource(stream)', $value['stream']);
        self::assertMatchesRegularExpression('~^Closure defined in .*SerializerTest\.php:\d+$~', $value['closure']);
        self::assertSame(['__class' => 'DateTimeImmutable', 'value' => '2026-10-19T12:00:00+03:00'], $value['date']);
        self::assertSame('RuntimeException', $value['error']['__class']);
        self::assertSame('boom', $value['error']['message']);
        self::assertSame(5, $value['error']['code']);
        self::assertStringEndsWith('SerializerTest.php', $value['error']['file']);
    }
}

class SampleBase
{
    // @phpstan-ignore property.onlyWritten (the serializer reads it through the array cast)
    private string $parentNote = 'base';
}

final class SampleUser extends SampleBase
{
    public string $name;
    // @phpstan-ignore property.onlyWritten (the serializer reads it through the array cast)
    private string $password;
    protected string $role = 'admin';

    public function __construct(string $name, string $password)
    {
        $this->name = $name;
        $this->password = $password;
    }
}
