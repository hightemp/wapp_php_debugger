<?php

declare(strict_types=1);

namespace WappDebugger;

interface Clock
{
    /** Milliseconds since the epoch. */
    public function now(): int;
}
