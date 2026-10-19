<?php

declare(strict_types=1);

namespace WappDebugger\Tests\Dev;

use PHPUnit\Framework\TestCase;
use WappDebugger\Dev\NamingChecker;
use WappDebugger\Dev\NamingViolation;

final class NamingCheckerTest extends TestCase
{
    /**
     * @return list<string>
     */
    private function names(string $source): array
    {
        return array_map(
            static function (NamingViolation $violation): string {
                return $violation->name;
            },
            (new NamingChecker())->check($source)
        );
    }

    public function testCleanCodeHasNoViolations(): void
    {
        $source = <<<'PHP'
            <?php
            final class Logger
            {
                private string $logsDir = '';
                public function isEnabled(bool $force): bool
                {
                    $items = [];
                    $url = $this->logsDir;
                    return $force || $items === [] || $url === '';
                }
            }
            PHP;

        self::assertSame([], $this->names($source));
    }

    /**
     * @dataProvider hungarianVariables
     */
    public function testHungarianVariablesAreReported(string $variable): void
    {
        self::assertSame([$variable], $this->names("<?php\n\${$variable} = 1;\n"));
    }

    /**
     * @return iterable<string, array{string}>
     */
    public function hungarianVariables(): iterable
    {
        yield 'array' => ['aVars'];
        yield 'string' => ['sMessage'];
        yield 'int' => ['iCount'];
        yield 'bool' => ['bIsOnFlag'];
        yield 'object' => ['oProject'];
        yield 'mixed' => ['mValue'];
        yield 'float' => ['fRatio'];
        yield 'function' => ['fnCallback'];
    }

    public function testWordsThatOnlyStartWithPrefixLetterAreAllowed(): void
    {
        $source = "<?php\n\$items = \$message = \$index = \$open = \$isValid = \$fn = \$a = \$b = 1;\n";

        self::assertSame([], $this->names($source));
    }

    public function testPropertiesParametersAndMethodsAreReported(): void
    {
        $source = <<<'PHP'
            <?php
            class Debugger
            {
                public $sLogsDir;
                public function fnDebugLog($sType) {}
            }
            function fnGetInstance() {}
            PHP;

        self::assertSame(['sLogsDir', 'fnDebugLog', 'sType', 'fnGetInstance'], $this->names($source));
    }

    public function testEachNameIsReportedOncePerLine(): void
    {
        self::assertSame(['sName'], $this->names("<?php\n\$sName = \$sName . \$sName;\n"));
    }

    public function testViolationHasLine(): void
    {
        $violations = (new NamingChecker())->check("<?php\n\n\$ok = 1;\n\$sBad = 2;\n");

        self::assertCount(1, $violations);
        self::assertSame(4, $violations[0]->line);
    }

    public function testSuperglobalsThisAndAnonymousFunctionsAreIgnored(): void
    {
        $source = "<?php\n\$x = \$_SERVER + \$GLOBALS;\n\$f = function () { return \$this; };\n\$g = fn() => 1;\n";

        self::assertSame([], $this->names($source));
    }

    public function testUsagesOfForeignPropertiesAndMethodsAreNotReported(): void
    {
        // Чужой API (например, старой библиотеки) можно вызывать: проверяются только наши объявления и переменные
        $source = "<?php\n\$debugger->fnDebugLog(\$debugger->sMessagesFilesPath);\nDebugger::fnGetInstance();\n";

        self::assertSame([], $this->names($source));
    }

    public function testImportedFunctionsAreNotReported(): void
    {
        self::assertSame([], $this->names("<?php\nuse function fnDebugLog;\nfnDebugLog();\n"));
    }

    public function testLineCanBeExcludedWithIgnoreComment(): void
    {
        // Нужно для deprecated-обёрток со старыми именами (п. 10.23 плана)
        $source = "<?php\nfunction fnDebugLog() {} // naming-check:ignore\nfunction fnOther() {}\n";

        self::assertSame(['fnOther'], $this->names($source));
    }
}
