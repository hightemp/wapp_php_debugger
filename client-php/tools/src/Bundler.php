<?php

declare(strict_types=1);

namespace WappDebugger\Dev;

/**
 * Builds the one-file library: every class of src/ in a bracketed namespace block and the global
 * helpers last, for projects without Composer (one require). The PSR-3 and Monolog bridges stay
 * out: they extend classes of Composer packages, and declaring them without those would be fatal.
 */
final class Bundler
{
    private const SKIPPED = ['autoload.php'];
    private const SKIPPED_DIRS = ['Bridge'];

    public function build(string $srcDir, string $target): void
    {
        $files = $this->sourceFiles($srcDir);
        $version = $this->version($srcDir);

        $out = "<?php\n\n"
            . "/*\n * wapp-debugger PHP library {$version}, one-file build of client-php/src.\n"
            . " * https://github.com/hightemp/wapp_php_debugger — MIT license.\n"
            . " * Usage: require __DIR__ . '/wapp-debugger.php'; WappDebugger\\Debugger::boot(__DIR__ . '/dws_debugger.json');\n */\n\n"
            . "declare(strict_types=1);\n";
        foreach ($files as $file) {
            $out .= "\n" . $this->block($file);
        }

        $dir = dirname($target);
        if (!is_dir($dir) && !mkdir($dir, 0775, true) && !is_dir($dir)) {
            throw new \RuntimeException("cannot create $dir");
        }
        if (file_put_contents($target, $out) === false) {
            throw new \RuntimeException("cannot write $target");
        }
    }

    /**
     * Interfaces first (classes implementing them must find them declared), the helpers last.
     *
     * @return list<string>
     */
    private function sourceFiles(string $srcDir): array
    {
        $interfaces = [];
        $classes = [];
        $functions = [];
        $items = new \RecursiveIteratorIterator(new \RecursiveDirectoryIterator($srcDir, \FilesystemIterator::SKIP_DOTS));
        foreach ($items as $item) {
            /** @var \SplFileInfo $item */
            $relative = substr($item->getPathname(), strlen(rtrim($srcDir, '/')) + 1);
            if ($item->getExtension() !== 'php' || in_array($relative, self::SKIPPED, true)
                || in_array(explode('/', $relative)[0], self::SKIPPED_DIRS, true)) {
                continue;
            }
            $source = (string) file_get_contents($item->getPathname());
            if ($relative === 'functions.php') {
                $functions[] = $item->getPathname();
            } elseif (preg_match('/^interface\s/m', $source) === 1) {
                $interfaces[] = $item->getPathname();
            } else {
                $classes[] = $item->getPathname();
            }
        }
        sort($interfaces);
        sort($classes);

        return array_merge($interfaces, $classes, $functions);
    }

    private function block(string $file): string
    {
        $namespace = '';
        $body = '';
        $tokens = token_get_all((string) file_get_contents($file));
        $count = count($tokens);
        for ($i = 0; $i < $count; ++$i) {
            $token = $tokens[$i];
            if (is_array($token) && $token[0] === T_OPEN_TAG) {
                continue;
            }
            if (is_array($token) && $token[0] === T_DECLARE) {
                // declare(strict_types=1); the bundle declares it once at the top
                while ($i < $count && $tokens[$i] !== ';') {
                    ++$i;
                }
                continue;
            }
            // The namespace declaration comes before any code; later T_NAMESPACE tokens are namespace\name()
            if (is_array($token) && $token[0] === T_NAMESPACE && trim($body) === '') {
                $name = '';
                for (++$i; $i < $count && $tokens[$i] !== ';'; ++$i) {
                    $name .= is_array($tokens[$i]) ? $tokens[$i][1] : $tokens[$i];
                }
                $namespace = trim($name);
                continue;
            }
            $body .= is_array($token) ? $token[1] : $token;
        }
        $source = trim($body);
        if (basename($file) === 'Frames.php') {
            $flag = 'private const BUNDLED = false;';
            if (strpos($source, $flag) === false) {
                throw new \RuntimeException('Frames.php has no BUNDLED flag');
            }
            $source = str_replace($flag, 'private const BUNDLED = true;', $source);
        }

        return sprintf("namespace %s{\n\n%s\n\n}\n", $namespace === '' ? '' : $namespace . ' ', $source);
    }

    private function version(string $srcDir): string
    {
        if (preg_match("/public const VERSION = '([^']+)';/", (string) file_get_contents($srcDir . '/Debugger.php'), $match) !== 1) {
            throw new \RuntimeException('no Debugger::VERSION');
        }

        return $match[1];
    }
}
