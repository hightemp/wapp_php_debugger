<?php

declare(strict_types=1);

namespace WappDebugger;

/**
 * Pushes the events to the receiver of the viewer (spec/ingest.yaml) instead of a file:
 * in batches, the rest when the request ends, and what it has every second in a long run.
 * It needs no extension: PHP streams send the batch (https needs openssl, as for any https).
 * One failed send stops logging, so a missing viewer never slows the application down twice.
 */
final class HttpWriter implements Writer
{
    private const TIMEOUT_SECONDS = 2.0;
    private const BATCH_BYTES = 512 * 1024;

    private string $url;
    private string $token;
    /** @var callable(string, string, string): bool */
    private $send;
    /** @var callable(): float */
    private $now;
    private int $batchLines;
    private string $buffer = '';
    private int $lines = 0;
    private float $since = 0.0;
    private bool $failed = false;

    /**
     * @param (callable(string, string, string): bool)|null $send url, token and body; true when taken
     * @param (callable(): float)|null                       $now  seconds, for tests
     */
    public function __construct(string $url, string $token, ?callable $send = null, int $batchLines = 100, ?callable $now = null)
    {
        $this->url = $url;
        $this->token = $token;
        $this->send = $send ?? [self::class, 'post'];
        $this->batchLines = max(1, $batchLines);
        $this->now = $now ?? static function (): float {
            return microtime(true);
        };
    }

    public function write(string $line): bool
    {
        if ($this->failed) {
            return false;
        }
        if ($this->lines === 0) {
            $this->since = ($this->now)();
        }
        $this->buffer .= $line . "\n";
        ++$this->lines;
        if ($this->lines >= $this->batchLines || strlen($this->buffer) >= self::BATCH_BYTES || ($this->now)() - $this->since >= 1.0) {
            return $this->flush();
        }

        return true;
    }

    public function close(): void
    {
        if (!$this->failed && $this->lines > 0) {
            $this->flush();
        }
    }

    private function flush(): bool
    {
        $body = $this->buffer;
        $this->buffer = '';
        $this->lines = 0;
        if (!($this->send)($this->url, $this->token, $body)) {
            $this->failed = true;

            return false;
        }

        return true;
    }

    /** POSTs a batch with PHP streams; true for a 2xx answer. */
    public static function post(string $url, string $token, string $body): bool
    {
        $context = stream_context_create(['http' => [
            'method' => 'POST',
            'header' => ['Content-Type: application/x-ndjson', 'Authorization: Bearer ' . $token],
            'content' => $body,
            'timeout' => self::TIMEOUT_SECONDS,
            'ignore_errors' => true,
            'follow_location' => 0,
        ]]);
        $stream = @fopen($url, 'rb', false, $context);
        if ($stream === false) {
            return false;
        }
        $meta = stream_get_meta_data($stream);
        fclose($stream);
        $status = is_array($meta['wrapper_data'] ?? null) ? (string) ($meta['wrapper_data'][0] ?? '') : '';

        return preg_match('~^HTTP/\S+ 2\d\d~', $status) === 1;
    }
}
