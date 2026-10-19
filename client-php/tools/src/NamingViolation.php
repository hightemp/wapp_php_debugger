<?php

declare(strict_types=1);

namespace WappDebugger\Dev;

final class NamingViolation
{
    public string $name;

    public int $line;

    public function __construct(string $name, int $line)
    {
        $this->name = $name;
        $this->line = $line;
    }
}
