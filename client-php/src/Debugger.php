<?php

declare(strict_types=1);

namespace WappDebugger;

/**
 * One request (or CLI process): writes its v2 events into <start ts>-<request_id>.jsonl.
 * A disabled debugger does nothing; no method ever throws into the application.
 */
final class Debugger
{
    public const VERSION = '2.0.0';

    private const JSON_FLAGS = JSON_UNESCAPED_UNICODE | JSON_UNESCAPED_SLASHES | JSON_INVALID_UTF8_SUBSTITUTE
        | JSON_PRESERVE_ZERO_FRACTION | JSON_PARTIAL_OUTPUT_ON_ERROR;

    private static ?self $instance = null;

    private Clock $clock;
    private Serializer $serializer;
    private Masker $masker;
    private Frames $frames;
    private int $traceFrames;
    private float $startedMonotonic;
    /** @var array<string, float> running named timers: name => monotonic start */
    private array $timers = [];
    /** @var list<float> running unnamed timers, a stack as in library v1 */
    private array $unnamedTimers = [];
    private ?Writer $writer;
    private string $requestId;
    private int $lastTs;
    private int $seq = 0;
    private bool $cli;
    private bool $finished = false;
    /** @var array{string, string}|null the JS client address and the push token, when pushing */
    private ?array $jsClient = null;

    private function __construct(Clock $clock, Serializer $serializer, Masker $masker, Frames $frames, int $traceFrames, ?Writer $writer, string $requestId, int $startedAt, bool $cli)
    {
        $this->clock = $clock;
        $this->serializer = $serializer;
        $this->masker = $masker;
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
        $writer = null;
        $pushUrl = $config->pushUrl();
        $pushToken = $config->pushToken();
        $jsClient = null;
        if ($config->enabled() && $pushUrl !== null && $pushToken !== null) {
            $writer = new HttpWriter($pushUrl, $pushToken);
            // The receiver serves the JS client next to its endpoint: …/ingest/v2 → …/ingest/debugger.js
            $jsClient = [(string) preg_replace('~/[^/]*$~', '/debugger.js', $pushUrl), $pushToken];
        } elseif ($config->enabled() && $logsDir !== null) {
            $fileName = $now . '-' . $requestId . '.jsonl';
            $rotation = new Rotation($config->maxAgeDays(), $config->maxFiles(), $config->maxLogsBytes());
            if ($rotation->due($logsDir, $now)) {
                $rotation->run($logsDir, $now, $fileName);
            }
            $writer = new JsonlFileWriter($logsDir, $fileName, $config->fileMode(), $config->dirMode());
        }

        $masker = new Masker($config->maskKeys());
        $serializer = new Serializer($masker, $config->maxDepth(), $config->maxItems(), $config->maxStringBytes());
        $debugger = new self($clock, $serializer, $masker, new Frames($config->projectRoot()), $config->traceFrames(), $writer, $requestId, $now, $request->isCli());
        $debugger->jsClient = $jsClient;
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
        // The HTTP mode works with logging off too: that is how it gets turned on remotely
        if ($config->httpToken() !== null && PHP_SAPI !== 'cli') {
            $response = (new HttpEndpoint($configPath, $config))->handle($_SERVER);
            if ($response !== null) {
                $response->send();

                exit;
            }
        }
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

    /**
     * The <script> tag of the JS client: errors of the page go to the viewer as a child of this
     * request. Empty unless the request is pushed (push_url): the page needs the receiver and the token.
     * The token is visible to anyone who opens the page, so this is for dev and stage.
     */
    public function jsTag(): string
    {
        if ($this->jsClient === null || !$this->enabled()) {
            return '';
        }
        [$src, $token] = $this->jsClient;
        $escape = static function (string $value): string {
            return htmlspecialchars($value, ENT_QUOTES | ENT_SUBSTITUTE, 'UTF-8');
        };

        return sprintf('<script src="%s" data-token="%s" data-request-id="%s" async></script>', $escape($src), $escape($token), $escape($this->requestId));
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
        $this->write($level, $message, $context, $tags, null);
    }

    /**
     * An SQL query that has run: the SQL tab of the viewer lists queries with their time and finds
     * repeated ones (N+1). Parameters are masked like any context.
     *
     * @param array<array-key, mixed> $params
     */
    public function sql(string $sql, array $params = [], ?float $durationMs = null, ?int $rows = null): void
    {
        $context = $params === [] ? [] : ['params' => $params];
        if ($rows !== null) {
            $context['rows'] = $rows;
        }
        $this->write(Level::DEBUG, $sql, $context, ['sql'], $durationMs);
    }

    /**
     * An outgoing HTTP request that has ended: a response status, or the error that stopped it.
     * Secret query parameters of the URL are masked.
     */
    public function http(string $method, string $url, ?int $status = null, ?float $durationMs = null, ?string $error = null): void
    {
        $context = $status !== null ? ['status' => $status] : [];
        if ($error !== null) {
            $context['error'] = $error;
        }
        $failed = $error !== null || ($status !== null && $status >= 400);
        $this->write($failed ? Level::WARNING : Level::INFO, strtoupper($method) . ' ' . $this->masker->url($url), $context, ['http'], $durationMs);
    }

    /** A monotonic clock in ms, to time an operation for sql() and http(). */
    public function monotonicMs(): float
    {
        return $this->clock->monotonicMs();
    }

    /**
     * @param array<array-key, mixed> $context
     * @param list<string>            $tags
     */
    private function write(string $level, string $message, array $context, array $tags, ?float $durationMs): void
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
        if ($durationMs !== null) {
            $fields['duration_ms'] = round(max(0.0, $durationMs), 3);
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
     * @param array<array-key, mixed> $context
     * @param list<string>            $tags
     */
    public function error(string $message, array $context = [], array $tags = []): void
    {
        $this->log(Level::ERROR, $message, $context, $tags);
    }

    /**
     * @param array<array-key, mixed> $context
     * @param list<string>            $tags
     */
    public function warning(string $message, array $context = [], array $tags = []): void
    {
        $this->log(Level::WARNING, $message, $context, $tags);
    }

    /**
     * @param array<array-key, mixed> $context
     * @param list<string>            $tags
     */
    public function info(string $message, array $context = [], array $tags = []): void
    {
        $this->log(Level::INFO, $message, $context, $tags);
    }

    /**
     * @param array<array-key, mixed> $context
     * @param list<string>            $tags
     */
    public function message(string $message, array $context = [], array $tags = []): void
    {
        $this->log(Level::MESSAGE, $message, $context, $tags);
    }

    /**
     * @param array<array-key, mixed> $context
     * @param list<string>            $tags
     */
    public function debug(string $message, array $context = [], array $tags = []): void
    {
        $this->log(Level::DEBUG, $message, $context, $tags);
    }

    /**
     * Writes an error event of a PHP error or a fatal error; used by ErrorHandler.
     *
     * @internal
     *
     * @param array<string, mixed>           $fields kind, level, message, error_type
     * @param array{file: string, line: int} $source
     * @param list<array<string, mixed>>     $backtrace
     */
    public function phpError(array $fields, array $source, array $backtrace): void
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
