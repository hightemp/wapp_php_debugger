<?php

declare(strict_types=1);

namespace WappDebugger;

final class SystemClock implements Clock
{
    public function now(): int
    {
        return (int) floor(microtime(true) * 1000);
    }

    public function monotonicMs(): float
    {
        return hrtime(true) / 1e6;
    }
}
