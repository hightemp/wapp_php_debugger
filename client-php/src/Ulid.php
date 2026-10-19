<?php

declare(strict_types=1);

namespace WappDebugger;

/**
 * ULID: 48 bits of milliseconds and 80 random bits in Crockford base32, so ids sort by time.
 */
final class Ulid
{
    private const ALPHABET = '0123456789ABCDEFGHJKMNPQRSTVWXYZ';

    private function __construct()
    {
    }

    /**
     * @param string|null $random 10 bytes of randomness; null takes them from random_bytes()
     */
    public static function generate(int $milliseconds, ?string $random = null): string
    {
        $random ??= random_bytes(10);

        $time = '';
        for ($i = 0; $i < 10; ++$i) {
            $time = self::ALPHABET[$milliseconds % 32] . $time;
            $milliseconds = intdiv($milliseconds, 32);
        }

        // 80 bits = 16 groups of 5 bits, taken from the bytes most significant first
        $bits = '';
        foreach (str_split($random) as $byte) {
            $bits .= str_pad(decbin(ord($byte)), 8, '0', STR_PAD_LEFT);
        }
        $tail = '';
        foreach (str_split($bits, 5) as $group) {
            $tail .= self::ALPHABET[(int) bindec($group)];
        }

        return $time . $tail;
    }
}
