<?php

declare(strict_types=1);

namespace WappDebugger;

final class HttpResponse
{
    private int $status;
    /** @var array<string, string> */
    private array $headers;
    private string $body;

    /**
     * @param array<string, string> $headers
     */
    public function __construct(int $status, array $headers, string $body)
    {
        $this->status = $status;
        $this->headers = $headers;
        $this->body = $body;
    }

    /**
     * @param array<string, mixed> $data
     */
    public static function json(int $status, array $data): self
    {
        return new self($status, ['Content-Type' => 'application/json'], (string) json_encode($data, JSON_UNESCAPED_SLASHES));
    }

    /**
     * @param array<string, string> $headers
     */
    public static function problem(int $status, string $title, string $detail, array $headers = []): self
    {
        return new self(
            $status,
            ['Content-Type' => 'application/problem+json'] + $headers,
            (string) json_encode(['title' => $title, 'status' => $status, 'detail' => $detail], JSON_UNESCAPED_SLASHES)
        );
    }

    public function status(): int
    {
        return $this->status;
    }

    /**
     * @return array<string, string>
     */
    public function headers(): array
    {
        return $this->headers;
    }

    public function body(): string
    {
        return $this->body;
    }

    public function send(): void
    {
        if (!headers_sent()) {
            http_response_code($this->status);
            header('Cache-Control: no-store');
            foreach ($this->headers as $name => $value) {
                header($name . ': ' . $value);
            }
        }
        echo $this->body;
    }
}
