<?php

declare(strict_types=1);

namespace WappDebugger\Bridge;

use Monolog\Handler\AbstractProcessingHandler;
use Monolog\Logger;
use Monolog\LogRecord;
use WappDebugger\Debugger;
use WappDebugger\Frames;

/**
 * Monolog 3 handler; use MonologHandler, which picks the class for the installed Monolog.
 */
final class Monolog3Handler extends AbstractProcessingHandler
{
    private Debugger $debugger;

    /**
     * @param int|string|\Monolog\Level $level the minimal level, as in any Monolog handler
     */
    public function __construct(?Debugger $debugger = null, $level = 100, bool $bubble = true)
    {
        parent::__construct($level, $bubble);
        $this->debugger = $debugger ?? Debugger::instance();
        Frames::skipDirectory(Levels::dirOf(Logger::class));
    }

    protected function write(LogRecord $record): void
    {
        $context = $record->context;
        if ($record->extra !== []) {
            $context['extra'] = $record->extra;
        }
        $this->debugger->log(Levels::PSR3[$record->level->toPsrLogLevel()], $record->message, $context, ['monolog', $record->channel]);
        if (($record->context['exception'] ?? null) instanceof \Throwable) {
            $this->debugger->exception($record->context['exception']);
        }
    }
}
