<?php

declare(strict_types=1);

namespace WappDebugger\Tests;

use PHPUnit\Framework\TestCase;
use WappDebugger\Tests\Support\LogAssert;
use WappDebugger\Tests\Support\TempDir;

/**
 * Runs real PHP processes: the handlers must work where PHPUnit is not around.
 */
final class ProcessTest extends TestCase
{
    use TempDir;

    /**
     * @param array<string, mixed>|null $config
     *
     * @return array{int, string, list<array<string, mixed>>} exit code, output and log events
     */
    private function runScript(string $script, ?array $config = null): array
    {
        $config ??= ['enabled' => true, 'logs_dir' => $this->tempDir() . '/logs', 'project_root' => dirname(__DIR__)];
        file_put_contents($this->tempDir() . '/dws_debugger.json', json_encode($config));
        $command = sprintf('%s -d display_errors=stderr %s %s 2>&1', escapeshellarg(PHP_BINARY), escapeshellarg(__DIR__ . '/scripts/' . $script), escapeshellarg($this->tempDir()));
        exec($command, $output, $exitCode);
        $files = $this->logFiles($this->tempDir() . '/logs');
        self::assertCount(1, $files, implode("\n", $output));

        return [$exitCode, implode("\n", $output), LogAssert::validStream($files[0])];
    }

    public function testWarningAndUncaughtException(): void
    {
        [$exitCode, $output, $events] = $this->runScript('errors.php');

        self::assertSame(255, $exitCode);
        self::assertStringContainsString('Order 7 not found', $output, 'PHP still reports the uncaught exception');
        self::assertStringContainsString('Undefined index: discount', $output, 'and the warning');
        self::assertSame(['request_start', 'message', 'error', 'error', 'request_end'], array_column($events, 'type'));
        self::assertSame('E_USER_WARNING', $events[2]['error_type']);
        self::assertSame('tests/scripts/errors.php', $events[2]['source']['file']);
        self::assertSame('RuntimeException', $events[3]['exception_class']);
        self::assertSame('load', $events[3]['trace'][0]['function']);
        self::assertSame(255, $events[4]['exit_code']);
    }

    public function testFatalError(): void
    {
        [$exitCode, $output, $events] = $this->runScript('fatal.php');

        self::assertSame(255, $exitCode);
        self::assertStringContainsString('Allowed memory size', $output);
        self::assertSame(['request_start', 'error', 'request_end'], array_column($events, 'type'));
        self::assertSame('fatal', $events[1]['kind']);
        self::assertSame('E_ERROR', $events[1]['error_type']);
        self::assertSame(255, $events[2]['exit_code']);
    }

    public function testProjectOfLibraryV1WithoutComposer(): void
    {
        [$exitCode, $output, $events] = $this->runScript('compat.php', [
            'bIsOnFlag' => true, 'sProjectRootPath' => dirname(__DIR__), 'sMessagesFilesPath' => $this->tempDir() . '/logs',
            'sTempPath' => $this->tempDir(),
        ]);

        self::assertSame(0, $exitCode, $output);
        self::assertSame(['request_start', 'message', 'message', 'message', 'timer', 'request_end'], array_column($events, 'type'));
        self::assertSame('warning', $events[1]['level']);
        self::assertSame(['config'], $events[1]['tags']);
        self::assertSame(['id' => 7, 'password' => '***'], $events[2]['context']);
        self::assertSame('tests/scripts/compat.php', $events[2]['source']['file']);
        self::assertSame('40', $events[3]['message']);
        self::assertSame('total', $events[4]['name']);
        self::assertArrayNotHasKey('exit_code', $events[5], 'a normal CLI exit code is not known at shutdown');
    }
}
