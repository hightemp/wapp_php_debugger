<?php

declare(strict_types=1);

namespace WappDebugger\Tests\Dev;

use PHPUnit\Framework\TestCase;
use WappDebugger\Dev\Bundler;
use WappDebugger\Tests\Support\LogAssert;
use WappDebugger\Tests\Support\TempDir;

/**
 * The one-file library: what a project without Composer gets with a single require.
 */
final class BundlerTest extends TestCase
{
    use TempDir;

    private function bundle(): string
    {
        $file = $this->tempDir() . '/lib/wapp-debugger.php';
        (new Bundler())->build(dirname(__DIR__, 2) . '/src', $file);

        return $file;
    }

    public function testIsOneValidPhpFileWithoutTheBridges(): void
    {
        $file = $this->bundle();
        exec(escapeshellarg(PHP_BINARY) . ' -l ' . escapeshellarg($file) . ' 2>&1', $output, $code);
        $source = (string) file_get_contents($file);

        self::assertSame(0, $code, implode("\n", $output));
        self::assertSame(1, substr_count($source, '<?php'));
        self::assertSame(1, substr_count($source, 'declare(strict_types=1)'));
        self::assertStringContainsString('wapp-debugger PHP library ' . \WappDebugger\Debugger::VERSION, $source);
        self::assertStringNotContainsString('class PsrLogger', $source, 'the bridges need Composer packages');
        self::assertStringNotContainsString('spl_autoload_register', $source);
    }

    public function testWorksWithASingleRequire(): void
    {
        $bundle = $this->bundle();
        file_put_contents($this->tempDir() . '/dws_debugger.json', json_encode(['enabled' => true, 'logs_dir' => 'logs', 'trace_frames' => 2]));
        // A neighbour of the bundle: code next to it is the application, not the library
        copy(__DIR__ . '/../scripts/bundle/app.php', $this->tempDir() . '/lib/app.php');

        // -n: no php.ini, so no extensions beyond the ones built into PHP — the library needs nothing more
        $command = sprintf('%s -n %s %s 2>&1', escapeshellarg(PHP_BINARY), escapeshellarg($this->tempDir() . '/lib/app.php'), escapeshellarg($this->tempDir()));
        exec($command, $output, $code);
        $files = $this->logFiles($this->tempDir() . '/logs');

        self::assertSame(255, $code, implode("\n", $output));
        self::assertCount(1, $files);
        $events = LogAssert::validStream($files[0]);
        self::assertSame(['request_start', 'message', 'timer', 'error', 'request_end'], array_column($events, 'type'));
        self::assertSame('lib/app.php', $events[1]['source']['file'], 'the call site is the application, next to the bundle');
        self::assertSame(['id' => 7, 'password' => '***'], $events[1]['context']);
        self::assertSame('LogicException', $events[3]['exception_class']);
    }
}
