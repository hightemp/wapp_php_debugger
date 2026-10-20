<?php

declare(strict_types=1);

namespace WappDebugger\Tests;

use PHPUnit\Framework\TestCase;

/**
 * The library is a logger script without dependencies: it drops into any PHP 7.4+ project, legacy
 * ones included. PSR-3 and Monolog are optional bridges for projects that have them already.
 */
final class PackageTest extends TestCase
{
    public function testRequiresNothingButPhp(): void
    {
        $composer = json_decode((string) file_get_contents(dirname(__DIR__) . '/composer.json'), true);

        self::assertIsArray($composer);
        // ext-json is part of PHP (always built in since 8.0; on 7.4 a build may load it as a module)
        self::assertSame(['php', 'ext-json'], array_keys($composer['require']), 'a dependency of the library would be a dependency of every project');
    }
}
