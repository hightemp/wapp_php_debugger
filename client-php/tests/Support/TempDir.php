<?php

declare(strict_types=1);

namespace WappDebugger\Tests\Support;

trait TempDir
{
    private string $tempDir = '';

    protected function tempDir(): string
    {
        if ($this->tempDir === '') {
            $this->tempDir = sys_get_temp_dir() . '/wapp-debugger-test-' . bin2hex(random_bytes(6));
            mkdir($this->tempDir, 0700, true);
        }

        return $this->tempDir;
    }

    /**
     * @after
     */
    protected function removeTempDir(): void
    {
        if ($this->tempDir === '' || !is_dir($this->tempDir)) {
            return;
        }
        $items = new \RecursiveIteratorIterator(
            new \RecursiveDirectoryIterator($this->tempDir, \FilesystemIterator::SKIP_DOTS),
            \RecursiveIteratorIterator::CHILD_FIRST
        );
        foreach ($items as $item) {
            /** @var \SplFileInfo $item */
            $item->isDir() ? rmdir($item->getPathname()) : unlink($item->getPathname());
        }
        rmdir($this->tempDir);
        $this->tempDir = '';
    }

    /**
     * @return list<string>
     */
    protected function logFiles(string $dir): array
    {
        $files = glob($dir . '/*.jsonl');

        return $files === false ? [] : $files;
    }
}
