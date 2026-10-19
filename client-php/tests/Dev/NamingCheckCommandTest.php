<?php

declare(strict_types=1);

namespace WappDebugger\Tests\Dev;

use PHPUnit\Framework\TestCase;
use WappDebugger\Dev\NamingCheckCommand;

final class NamingCheckCommandTest extends TestCase
{
    private string $dir;

    protected function setUp(): void
    {
        $this->dir = sys_get_temp_dir().'/naming-check-'.bin2hex(random_bytes(4));
        mkdir($this->dir.'/nested', 0777, true);
    }

    protected function tearDown(): void
    {
        foreach (['nested/bad.php', 'nested/notes.txt', 'good.php'] as $file) {
            @unlink($this->dir.'/'.$file);
        }
        @rmdir($this->dir.'/nested');
        @rmdir($this->dir);
    }

    /**
     * @param list<string> $paths
     *
     * @return array{int, string}
     */
    private function check(array $paths): array
    {
        $output = fopen('php://memory', 'w+');
        self::assertIsResource($output);

        $code = (new NamingCheckCommand())->run($paths, $output);
        rewind($output);

        return [$code, (string) stream_get_contents($output)];
    }

    public function testCleanFilesPass(): void
    {
        file_put_contents($this->dir.'/good.php', "<?php\n\$items = [];\n");

        [$code, $output] = $this->check([$this->dir]);

        self::assertSame(0, $code);
        self::assertStringContainsString('OK', $output);
    }

    public function testViolationsInNestedPhpFilesFailWithLocation(): void
    {
        file_put_contents($this->dir.'/good.php', "<?php\n\$items = [];\n");
        file_put_contents($this->dir.'/nested/bad.php', "<?php\n\n\$sName = 1;\n");
        file_put_contents($this->dir.'/nested/notes.txt', '$sIgnoredBecauseNotPhp');

        [$code, $output] = $this->check([$this->dir]);

        self::assertSame(1, $code);
        self::assertStringContainsString($this->dir.'/nested/bad.php:3: sName', $output);
        self::assertStringNotContainsString('sIgnoredBecauseNotPhp', $output);
    }

    public function testSingleFileCanBeChecked(): void
    {
        file_put_contents($this->dir.'/good.php', "<?php\nfunction fnRun() {}\n");

        [$code, $output] = $this->check([$this->dir.'/good.php']);

        self::assertSame(1, $code);
        self::assertStringContainsString('good.php:2: fnRun', $output);
    }

    public function testMissingPathIsAnError(): void
    {
        [$code, $output] = $this->check([$this->dir.'/missing']);

        self::assertSame(2, $code);
        self::assertStringContainsString('not found', $output);
    }

    public function testNoPathsIsAnError(): void
    {
        [$code, $output] = $this->check([]);

        self::assertSame(2, $code);
        self::assertStringContainsString('Usage', $output);
    }
}
