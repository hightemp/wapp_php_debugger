<?php

declare(strict_types=1);

namespace WappDebugger;

/**
 * Appends lines to one log file. The file is only ever appended to, so the viewer can
 * continue reading from a saved offset (locally or over SFTP).
 */
final class JsonlFileWriter
{
    private string $path;
    private string $dir;
    private int $fileMode;
    private int $dirMode;
    /** @var resource|null */
    private $handle;
    private bool $failed = false;

    public function __construct(string $dir, string $name, int $fileMode, int $dirMode)
    {
        $this->dir = $dir;
        $this->path = $dir . '/' . $name;
        $this->fileMode = $fileMode;
        $this->dirMode = $dirMode;
    }

    /** Writes one line; false when the file cannot be written (the caller stops logging). */
    public function write(string $line): bool
    {
        if ($this->failed) {
            return false;
        }
        if ($this->handle === null && !$this->open()) {
            $this->failed = true;

            return false;
        }
        assert($this->handle !== null);
        // One fwrite per line on a file opened for appending: lines of concurrent writers do not interleave
        $written = @fwrite($this->handle, $line . "\n");
        if ($written === false) {
            $this->failed = true;

            return false;
        }

        return true;
    }

    public function close(): void
    {
        if ($this->handle !== null) {
            fclose($this->handle);
            $this->handle = null;
        }
    }

    private function open(): bool
    {
        if (!is_dir($this->dir)) {
            if (!@mkdir($this->dir, $this->dirMode, true) && !is_dir($this->dir)) {
                return false;
            }
            // mkdir is subject to the umask; PHP often runs as another user than the viewer
            @chmod($this->dir, $this->dirMode);
        }
        $created = !is_file($this->path);
        $handle = @fopen($this->path, 'ab');
        if ($handle === false) {
            return false;
        }
        if ($created) {
            @chmod($this->path, $this->fileMode);
        }
        $this->handle = $handle;

        return true;
    }
}
