<?php

declare(strict_types=1);

namespace WappDebugger\Tests;

use PHPUnit\Framework\TestCase;
use WappDebugger\Masker;

final class MaskerTest extends TestCase
{
    /**
     * @dataProvider secretKeys
     */
    public function testMasksTheKeysOfLibraryV1(string $key): void
    {
        self::assertTrue((new Masker())->isSecret($key), $key);
    }

    /**
     * @return iterable<array{string}>
     */
    public static function secretKeys(): iterable
    {
        foreach (['password', 'user_pass', 'passwd', 'PWD', 'client_secret', 'X-Auth-Token', 'api_key', 'apiKey', 'APP_KEY', 'private_key',
            'aws_access_key', 'Authorization', 'authorisation', 'PHP_AUTH_PW', 'HTTP_COOKIE', 'DATABASE_DSN', 'csrf_token'] as $key) {
            yield $key => [$key];
        }
    }

    public function testLeavesOrdinaryKeys(): void
    {
        $masker = new Masker();
        foreach (['id', 'name', 'passage', 'pwd_hint', 'tokenizer_name', 'email'] as $key) {
            self::assertSame($key === 'tokenizer_name', $masker->isSecret($key), $key); // "token" anywhere counts, as in v1
        }
    }

    public function testExtraKeysFromTheConfig(): void
    {
        $masker = new Masker(['card_number', 'cvv']);

        self::assertTrue($masker->isSecret('card_number'));
        self::assertTrue($masker->isSecret('CVV'));
        self::assertTrue($masker->isSecret('password'), 'the defaults stay');
    }
}
