<?php

declare(strict_types=1);

namespace WappDebugger\Tests;

use PHPUnit\Framework\TestCase;
use WappDebugger\Tests\Support\LogAssert;
use WappDebugger\Tests\Support\Receiver;
use WappDebugger\Tests\Support\TempDir;

/**
 * The files of integrations/ in real PHP processes, with the CMS around them faked in tests/scripts.
 */
final class IntegrationsTest extends TestCase
{
    use Receiver;
    use TempDir;

    /**
     * @return array{string, list<array<string, mixed>>} output and the pushed events
     */
    private function runIntegration(string $script): array
    {
        $output = [];
        $this->withReceiver($this->tempDir(), function (string $url) use ($script, &$output): void {
            $config = ['enabled' => true, 'project_root' => dirname(__DIR__), 'push_url' => $url, 'push_token' => str_repeat('p', 40)];
            file_put_contents($this->tempDir() . '/dws_debugger.json', json_encode($config));
            $command = sprintf('%s -d display_errors=stderr %s %s 2>&1', escapeshellarg(PHP_BINARY), escapeshellarg(__DIR__ . '/scripts/integrations/' . $script), escapeshellarg($this->tempDir()));
            exec($command, $output, $exitCode);
            self::assertSame(0, $exitCode, implode("\n", $output));
        });

        return [implode("\n", $output), LogAssert::validStream($this->tempDir() . '/pushed.jsonl')];
    }

    public function testWordPressMuPlugin(): void
    {
        [$output, $events] = $this->runIntegration('wordpress.php');

        self::assertMatchesRegularExpression('~<body><script src="http://127\.0\.0\.1:\d+/ingest/debugger\.js" data-token="p{40}" data-request-id="[0-9A-Z]{26}" async></script></body>~', $output, 'the pages link their errors to the request');
        self::assertSame(['request_start', 'message', 'request_end'], array_column($events, 'type'));
        self::assertSame('SELECT * FROM wp_posts WHERE ID = 7', $events[1]['message']);
        self::assertSame(['sql'], $events[1]['tags']);
        self::assertSame(12.3, (float) $events[1]['duration_ms']);
    }

    public function testBitrixInit(): void
    {
        [$output, $events] = $this->runIntegration('bitrix.php');

        self::assertMatchesRegularExpression('~<h1>Catalog</h1><script src="[^"]+/ingest/debugger\.js"[^>]+></script></body>~', $output);
        self::assertSame(['request_start', 'message', 'request_end'], array_column($events, 'type'));
        self::assertSame('SELECT * FROM b_iblock_element WHERE ID = 7', $events[1]['message']);
        self::assertSame(4.2, (float) $events[1]['duration_ms']);
    }
}
