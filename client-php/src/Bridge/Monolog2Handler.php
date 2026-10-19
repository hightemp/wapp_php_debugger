<?php

declare(strict_types=1);

namespace WappDebugger\Bridge;

use Monolog\Handler\AbstractProcessingHandler;
use Monolog\Logger;
use WappDebugger\Debugger;
use WappDebugger\Frames;

/**
 * Monolog 2 handler (PHP 7.4); use MonologHandler, which picks the class for the installed Monolog.
 * PHPStan skips this file: it is analyzed against Monolog 3, where write() takes a LogRecord.
 */
final class Monolog2Handler extends AbstractProcessingHandler
{
    private Debugger $debugger;

    /**
     * @param int|string $level the minimal level, as in any Monolog handler
     */
    public function __construct(?Debugger $debugger = null, $level = 100, bool $bubble = true)
    {
        parent::__construct($level, $bubble);
        $this->debugger = $debugger ?? Debugger::instance();
        Frames::skipDirectory(Levels::dirOf(Logger::class));
    }

    /**
     * @param array<string, mixed> $record
     */
    protected function write(array $record): void
    {
        $context = is_array($record['context'] ?? null) ? $record['context'] : [];
        $withExtra = $context;
        if (is_array($record['extra'] ?? null) && $record['extra'] !== []) {
            $withExtra['extra'] = $record['extra'];
        }
        $level = Levels::PSR3[strtolower((string) ($record['level_name'] ?? 'debug'))] ?? 'debug';
        $this->debugger->log($level, (string) ($record['message'] ?? ''), $withExtra, ['monolog', (string) ($record['channel'] ?? '')]);
        if (($context['exception'] ?? null) instanceof \Throwable) {
            $this->debugger->exception($context['exception']);
        }
    }
}
