<?php

declare(strict_types=1);

namespace WappDebugger\Dev;

use FilesystemIterator;
use RecursiveDirectoryIterator;
use RecursiveIteratorIterator;
use SplFileInfo;

/**
 * Checks .php files under the given paths. Exit codes: 0 — clean, 1 — violations found, 2 — usage error.
 */
final class NamingCheckCommand
{
    private NamingChecker $checker;

    public function __construct(?NamingChecker $checker = null)
    {
        $this->checker = $checker ?? new NamingChecker();
    }

    /**
     * @param list<string> $paths
     * @param resource     $output
     */
    public function run(array $paths, $output): int
    {
        if ($paths === []) {
            fwrite($output, "Usage: check-naming <path>...\n");

            return 2;
        }

        $violationCount = 0;

        foreach ($paths as $path) {
            if (!file_exists($path)) {
                fwrite($output, "Path not found: {$path}\n");

                return 2;
            }

            foreach ($this->phpFiles($path) as $file) {
                foreach ($this->checker->check((string) file_get_contents($file)) as $violation) {
                    fwrite($output, "{$file}:{$violation->line}: {$violation->name}\n");
                    $violationCount++;
                }
            }
        }

        if ($violationCount > 0) {
            fwrite($output, "Found {$violationCount} name(s) in Hungarian notation, see CONTRIBUTING.md\n");

            return 1;
        }

        fwrite($output, "OK\n");

        return 0;
    }

    /**
     * @return list<string>
     */
    private function phpFiles(string $path): array
    {
        if (is_file($path)) {
            return [$path];
        }

        $files = [];
        $iterator = new RecursiveIteratorIterator(new RecursiveDirectoryIterator($path, FilesystemIterator::SKIP_DOTS));

        /** @var SplFileInfo $file */
        foreach ($iterator as $file) {
            if ($file->isFile() && $file->getExtension() === 'php') {
                $files[] = $file->getPathname();
            }
        }

        sort($files);

        return $files;
    }
}
