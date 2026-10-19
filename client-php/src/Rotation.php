<?php

declare(strict_types=1);

namespace WappDebugger;

/**
 * Keeps the logs directory within limits: age, number of files and total size. Only log files
 * of the library (v2 "<ts>-<ulid>.jsonl" and v1 "<ts>") are deleted, oldest first, by plain PHP.
 */
final class Rotation
{
    private const MARKER = '.rotation';
    /** Names of log files: v2 "<ts>-<ulid>.jsonl" and v1 "<ts>". */
    public const LOG_NAME = '/^(\d{10,16})(?:-[0-9A-HJKMNP-TV-Z]{26}\.jsonl)?$/';

    private int $maxAgeDays;
    private int $maxFiles;
    private int $maxBytes;
    private int $intervalSeconds;

    /**
     * @param int $maxAgeDays      0 means no age limit; likewise for the other limits
     * @param int $intervalSeconds how often a run is due: a request only checks the time of the last run
     */
    public function __construct(int $maxAgeDays, int $maxFiles, int $maxBytes, int $intervalSeconds = 60)
    {
        $this->maxAgeDays = $maxAgeDays;
        $this->maxFiles = $maxFiles;
        $this->maxBytes = $maxBytes;
        $this->intervalSeconds = $intervalSeconds;
    }

    public function due(string $dir, int $nowMs): bool
    {
        $marker = $dir . '/' . self::MARKER;
        clearstatcache(true, $marker);
        $last = @filemtime($marker);

        return $last === false || $last <= intdiv($nowMs, 1000) - $this->intervalSeconds;
    }

    /**
     * @param string|null $current the file of the running request, never deleted
     */
    public function run(string $dir, int $nowMs, ?string $current = null): void
    {
        $names = @scandir($dir);
        if ($names === false) {
            return;
        }
        @touch($dir . '/' . self::MARKER, intdiv($nowMs, 1000));

        $logs = [];
        foreach ($names as $name) {
            if (preg_match(self::LOG_NAME, $name, $match) === 1) {
                $logs[] = ['name' => $name, 'ts' => (int) $match[1], 'size' => (int) @filesize($dir . '/' . $name)];
            }
        }
        // Newest first: everything past a limit is older than what is kept
        usort($logs, static function (array $a, array $b): int {
            return $b['ts'] <=> $a['ts'];
        });

        $oldestAllowed = $this->maxAgeDays > 0 ? $nowMs - $this->maxAgeDays * 86400000 : null;
        $kept = 0;
        $keptBytes = 0;
        foreach ($logs as $log) {
            $tooOld = $oldestAllowed !== null && $log['ts'] < $oldestAllowed;
            $tooMany = $this->maxFiles > 0 && $kept >= $this->maxFiles;
            $tooBig = $this->maxBytes > 0 && $keptBytes + $log['size'] > $this->maxBytes;
            if ($log['name'] !== $current && ($tooOld || $tooMany || $tooBig)) {
                @unlink($dir . '/' . $log['name']);
                continue;
            }
            ++$kept;
            $keptBytes += $log['size'];
        }
    }
}
