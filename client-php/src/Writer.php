<?php

declare(strict_types=1);

namespace WappDebugger;

/**
 * Where the events of a request go: a log file, or the viewer over HTTP.
 */
interface Writer
{
    /** Takes one line; false means it cannot go on (the debugger stops logging). */
    public function write(string $line): bool;

    /** Sends or writes what is left; called at the end of the request. */
    public function close(): void;
}
