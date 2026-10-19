<?php

declare(strict_types=1);

namespace WappDebugger\Tests\Support;

use WappDebugger\Clock;

final class FixedClock implements Clock
{
    private int $now;

    public function __construct(int $now)
    {
        $this->now = $now;
    }

    public function now(): int
    {
        return $this->now;
    }

    public function monotonicMs(): float
    {
        return (float) $this->now;
    }

    public function advance(int $milliseconds): void
    {
        $this->now += $milliseconds;
    }
}
