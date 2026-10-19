<?php

declare(strict_types=1);

namespace WappDebugger;

/**
 * One request (or CLI process): writes its v2 events into <start ts>-<request_id>.jsonl.
 * A disabled debugger does nothing; no method ever throws into the application.
 */
final class Debugger
{
    public const VERSION = '2.0.0-dev';

    private const JSON_FLAGS = JSON_UNESCAPED_UNICODE | JSON_UNESCAPED_SLASHES | JSON_INVALID_UTF8_SUBSTITUTE
        | JSON_PRESERVE_ZERO_FRACTION | JSON_PARTIAL_OUTPUT_ON_ERROR;

    private Clock $clock;
    private Serializer $serializer;
    private ?JsonlFileWriter $writer;
    private string $requestId;
    private int $startedAt;
    private int $lastTs;
    private int $seq = 0;
    private bool $cli;
    private bool $finished = false;

    private function __construct(Clock $clock, Serializer $serializer, ?JsonlFileWriter $writer, string $requestId, int $startedAt, bool $cli)
    {
        $this->clock = $clock;
        $this->serializer = $serializer;
        $this->writer = $writer;
        $this->requestId = $requestId;
        $this->startedAt = $startedAt;
        $this->lastTs = $startedAt;
        $this->cli = $cli;
    }

    public static function start(Config $config, RequestInfo $request, ?Clock $clock = null): self
    {
        $clock ??= new SystemClock();
        $now = $clock->now();
        $requestId = Ulid::generate($now);
        $logsDir = $config->logsDir();
        $writer = $config->enabled() && $logsDir !== null
            ? new JsonlFileWriter($logsDir, $now . '-' . $requestId . '.jsonl', $config->fileMode(), $config->dirMode())
            : null;

        $serializer = new Serializer(new Masker($config->maskKeys()), $config->maxDepth(), $config->maxItems(), $config->maxStringBytes());
        $debugger = new self($clock, $serializer, $writer, $requestId, $now, $request->isCli());
        $debugger->emit('request_start', ['lang' => 'php', 'library_version' => self::VERSION, 'php_version' => PHP_VERSION] + $request->fields());

        return $debugger;
    }

    public function enabled(): bool
    {
        return $this->writer !== null && !$this->finished;
    }

    public function requestId(): string
    {
        return $this->requestId;
    }

    /**
     * @param array<array-key, mixed> $context
     */
    public function log(string $level, string $message, array $context = []): void
    {
        if (!$this->enabled()) {
            return;
        }
        $fields = ['level' => Level::normalize($level), 'message' => $this->serializer->normalize($message)];
        if ($context !== []) {
            $fields['context'] = (object) $this->serializer->normalize($context);
        }
        $this->emit('message', $fields);
    }

    /**
     * Writes request_end; later calls do nothing.
     *
     * @param int|null $code HTTP status for a web request, exit code for CLI
     */
    public function finish(?int $code = null): void
    {
        if (!$this->enabled()) {
            return;
        }
        $fields = ['duration_ms' => 0, 'memory_peak_bytes' => memory_get_peak_usage(true)];
        if ($code !== null) {
            if ($this->cli) {
                $fields['exit_code'] = $code;
            } elseif ($code >= 100 && $code <= 599) {
                $fields['status_code'] = $code;
            }
        }
        $this->emit('request_end', $fields);
        $this->finished = true;
        if ($this->writer !== null) {
            $this->writer->close();
        }
    }

    /**
     * @param array<string, mixed> $fields
     */
    private function emit(string $type, array $fields): void
    {
        if (!$this->enabled()) {
            return;
        }
        $this->lastTs = max($this->lastTs, $this->clock->now());
        if ($type === 'request_end') {
            $fields['duration_ms'] = $this->lastTs - $this->startedAt;
        }
        $event = ['v' => 2, 'type' => $type, 'request_id' => $this->requestId, 'seq' => $this->seq + 1, 'ts' => $this->lastTs] + $fields;
        $line = json_encode($event, self::JSON_FLAGS);
        assert($this->writer !== null);
        if ($line === false || !$this->writer->write($line)) {
            // Cannot write: stop logging rather than break the application
            $this->writer = null;

            return;
        }
        ++$this->seq;
    }
}
