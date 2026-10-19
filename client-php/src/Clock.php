<?php

declare(strict_types=1);

namespace WappDebugger;

interface Clock
{
    /** Milliseconds since the epoch. */
    public function now(): int;

    /** Milliseconds on a monotonic clock, for durations: unaffected by changes of the system time. */
    public function monotonicMs(): float;
}
