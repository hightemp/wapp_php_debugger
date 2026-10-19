<?php

declare(strict_types=1);

namespace WappDebugger\Tests;

use PHPUnit\Framework\TestCase;
use WappDebugger\Ulid;

final class UlidTest extends TestCase
{
    public function testEncodesTimeFirstInCrockfordBase32(): void
    {
        $ulid = Ulid::generate(1792400000000, str_repeat("\0", 10));

        self::assertSame(26, strlen($ulid));
        self::assertMatchesRegularExpression('/^[0-7][0-9A-HJKMNP-TV-Z]{25}$/', $ulid);
        self::assertSame('01M59NV100', substr($ulid, 0, 10), 'time 1792400000000 ms, computed independently');
        self::assertSame(str_repeat('0', 16), substr($ulid, 10), 'zero randomness');
        self::assertSame('7ZZZZZZZZZZZZZZZZZZZZZZZZZ', Ulid::generate(2 ** 48 - 1, str_repeat("\xff", 10)));
    }

    public function testRandomPartDiffers(): void
    {
        self::assertNotSame(Ulid::generate(1, null), Ulid::generate(1, null));
    }
}
