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
    private Frames $frames;
    private int $traceFrames;
    private float $startedMonotonic;
    /** @var array<string, float> running timers: name => monotonic start */
    private array $timers = [];
    private ?JsonlFileWriter $writer;
    private string $requestId;
    private int $lastTs;
    private int $seq = 0;
    private bool $cli;
    private bool $finished = false;

    private function __construct(Clock $clock, Serializer $serializer, Frames $frames, int $traceFrames, ?JsonlFileWriter $writer, string $requestId, int $startedAt, bool $cli)
    {
        $this->clock = $clock;
        $this->serializer = $serializer;
        $this->frames = $frames;
        $this->traceFrames = $traceFrames;
        $this->startedMonotonic = $clock->monotonicMs();
        $this->writer = $writer;
        $this->requestId = $requestId;
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
        $debugger = new self($clock, $serializer, new Frames($config->projectRoot()), $config->traceFrames(), $writer, $requestId, $now, $request->isCli());
        if ($writer !== null) {
            // Child processes (exec, proc_open, cron jobs started from here) link themselves to this request
            putenv(RequestInfo::PARENT_ENV . '=' . $requestId);
        }
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
     * @param array<array-key, mixed> $context named variables
     * @param list<string>            $tags    without #, for example sql
     */
    public function log(string $level, string $message, array $context = [], array $tags = []): void
    {
        if (!$this->enabled()) {
            return;
        }
        $fields = ['level' => Level::normalize($level), 'message' => $this->serializer->normalize($message)];
        $tags = self::tags($tags);
        if ($tags !== []) {
            $fields['tags'] = $tags;
        }
        if ($context !== []) {
            $fields['context'] = (object) $this->serializer->normalize($context);
        }
        $stack = $this->frames->callStack();
        if ($stack['source'] !== null) {
            $fields['source'] = $stack['source'];
        }
        if ($this->traceFrames > 0 && $stack['trace'] !== []) {
            $fields['trace'] = array_slice($stack['trace'], 0, $this->traceFrames);
        }
        $this->emit('message', $fields);
    }

    public function startTimer(string $name): void
    {
        if ($this->enabled()) {
            $this->timers[$name] = $this->clock->monotonicMs();
        }
    }

    /** Writes a timer event with the time since startTimer($name). */
    public function stopTimer(string $name, string $level = Level::MESSAGE): void
    {
        if (!$this->enabled()) {
            return;
        }
        if (!isset($this->timers[$name])) {
            $this->log(Level::WARNING, sprintf('Timer "%s" was stopped but never started', $name));

            return;
        }
        $fields = [
            'name' => $name === '' ? 'timer' : $name,
            'duration_ms' => round(max(0.0, $this->clock->monotonicMs() - $this->timers[$name]), 3),
            'level' => Level::normalize($level),
        ];
        unset($this->timers[$name]);
        $source = $this->frames->callStack()['source'];
        if ($source !== null) {
            $fields['source'] = $source;
        }
        $this->emit('timer', $fields);
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
     * @param list<string> $tags
     *
     * @return list<string>
     */
    private static function tags(array $tags): array
    {
        $result = [];
        foreach ($tags as $tag) {
            $tag = strtolower(ltrim((string) $tag, '#'));
            if (strlen($tag) <= 64 && preg_match('/^[a-z0-9][a-z0-9_.:-]*$/', $tag) === 1 && !in_array($tag, $result, true)) {
                $result[] = $tag;
            }
        }

        return $result;
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
            $fields['duration_ms'] = round(max(0.0, $this->clock->monotonicMs() - $this->startedMonotonic), 3);
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
