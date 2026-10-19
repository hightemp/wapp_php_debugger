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

    private static ?self $instance = null;

    private Clock $clock;
    private Serializer $serializer;
    private Frames $frames;
    private int $traceFrames;
    private float $startedMonotonic;
    /** @var array<string, float> running named timers: name => monotonic start */
    private array $timers = [];
    /** @var list<float> running unnamed timers, a stack as in library v1 */
    private array $unnamedTimers = [];
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

    /**
     * Reads the config next to the project and, when the debugger is on, starts the request,
     * registers the error handlers and finishes the request at shutdown. Off, it costs one is_file().
     */
    public static function boot(string $configPath): self
    {
        $config = Config::fromFile($configPath);
        if (!$config->enabled()) {
            return self::$instance = self::start($config, RequestInfo::cli([]));
        }
        $debugger = self::start($config, RequestInfo::fromGlobals($_SERVER));
        if ($config->oldKeys() !== []) {
            $debugger->log(Level::WARNING, 'dws_debugger.json uses keys of library v1; rename them', ['keys' => $config->oldKeys()], ['config']);
        }
        $globals = ['server' => $_SERVER, 'get' => $_GET, 'post' => $_POST, 'files' => $_FILES,
            'session' => $_SESSION ?? [], 'cookie' => $_COOKIE];
        foreach ($config->logGlobals() as $name) {
            if ($globals[$name] !== []) {
                $debugger->log(Level::MESSAGE, '$_' . strtoupper($name), $globals[$name], ['globals']);
            }
        }
        (new ErrorHandler($debugger, static function (): ?int {
            $code = http_response_code();

            return is_int($code) ? $code : null;
        }))->register();

        return self::$instance = $debugger;
    }

    /** The debugger started by boot(); a disabled one before that. */
    public static function instance(): self
    {
        return self::$instance ??= self::start(Config::fromArray([]), RequestInfo::cli([]));
    }

    /** Sets the debugger the global helpers (__d, __log, __start, __stop) write to; null resets it. */
    public static function setInstance(?self $debugger): void
    {
        self::$instance = $debugger;
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

    public function isCli(): bool
    {
        return $this->cli;
    }

    /** Writes an exception (with its chain of previous ones) as an error event with the full trace. */
    public function exception(\Throwable $exception): void
    {
        if (!$this->enabled()) {
            return;
        }
        $frames = $this->frames->ofThrowable($exception);
        $previous = [];
        for ($inner = $exception->getPrevious(); $inner !== null && count($previous) < 10; $inner = $inner->getPrevious()) {
            $previous[] = [
                'exception_class' => get_class($inner),
                'message' => $this->serializer->normalize($inner->getMessage()),
                'code' => $inner->getCode(),
                'source' => $this->frames->ofThrowable($inner)['source'],
            ];
        }
        $fields = [
            'kind' => 'exception',
            'level' => Level::ERROR,
            'message' => $this->serializer->normalize($exception->getMessage()),
            'exception_class' => get_class($exception),
            'code' => $exception->getCode(),
            'source' => $frames['source'],
            'trace' => $frames['trace'],
        ];
        if ($previous !== []) {
            $fields['previous'] = $previous;
        }
        $this->emit('error', $fields);
    }

    /**
     * Writes an error event; used by ErrorHandler.
     *
     * @internal
     *
     * @param array<string, mixed>       $fields kind, level, message, error_type
     * @param array{file: string, line: int} $source
     * @param list<array<string, mixed>> $backtrace
     */
    public function error(array $fields, array $source, array $backtrace): void
    {
        if (!$this->enabled()) {
            return;
        }
        $fields['message'] = $this->serializer->normalize($fields['message']);
        $fields['source'] = $this->frames->convert($source);
        $trace = array_map([$this->frames, 'convert'], $backtrace);
        if ($trace !== []) {
            $fields['trace'] = $trace;
        }
        $this->emit('error', $fields);
    }

    /** Starts a named timer; without a name the timers form a stack, as in library v1. */
    public function startTimer(string $name = ''): void
    {
        if (!$this->enabled()) {
            return;
        }
        if ($name === '') {
            $this->unnamedTimers[] = $this->clock->monotonicMs();
        } else {
            $this->timers[$name] = $this->clock->monotonicMs();
        }
    }

    /**
     * Writes a timer event with the time since startTimer($name). Without a level it follows the
     * duration, as in library v1: info, warning over 1 s, error over 2 s.
     */
    public function stopTimer(string $name = '', ?string $level = null): void
    {
        if (!$this->enabled()) {
            return;
        }
        if ($name === '' && $this->unnamedTimers !== []) {
            $name = (string) count($this->unnamedTimers);
            $started = (float) array_pop($this->unnamedTimers);
        } elseif ($name !== '' && isset($this->timers[$name])) {
            $started = $this->timers[$name];
            unset($this->timers[$name]);
        } else {
            $this->log(Level::WARNING, sprintf('Timer "%s" was stopped but never started', $name));

            return;
        }
        $duration = round(max(0.0, $this->clock->monotonicMs() - $started), 3);
        if ($level === null) {
            $level = $duration > 2000 ? Level::ERROR : ($duration > 1000 ? Level::WARNING : Level::INFO);
        }
        $fields = ['name' => $name, 'duration_ms' => $duration, 'level' => Level::normalize($level)];
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
