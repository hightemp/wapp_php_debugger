<?php

declare(strict_types=1);

namespace WappDebugger\Dev;

/**
 * Находит имена в венгерской нотации ($aVars, $sMessage, fnDebugLog) в объявлениях и переменных.
 * Использования чужих свойств и методов (->sName, ::fnName) не проверяются.
 */
final class NamingChecker
{
    private const HUNGARIAN_PATTERN = '/^(a|s|i|b|o|m|f|fn)[A-Z]/';

    private const IGNORE_MARKER = 'naming-check:ignore';

    private const IGNORED_VARIABLES = [
        'this', 'GLOBALS', '_SERVER', '_GET', '_POST', '_FILES', '_COOKIE', '_SESSION', '_REQUEST', '_ENV',
    ];

    /**
     * @return list<NamingViolation>
     */
    public function check(string $source): array
    {
        $tokens = token_get_all($source);
        $ignoredLines = $this->findIgnoredLines($tokens);
        $violations = [];
        $seen = [];

        foreach ($tokens as $index => $token) {
            if (!is_array($token)) {
                continue;
            }

            $name = $this->declaredName($tokens, $index);
            if ($name === null || !preg_match(self::HUNGARIAN_PATTERN, $name)) {
                continue;
            }

            $line = $token[2];
            $key = $line.':'.$name;
            if (isset($ignoredLines[$line]) || isset($seen[$key])) {
                continue;
            }

            $seen[$key] = true;
            $violations[] = new NamingViolation($name, $line);
        }

        return $violations;
    }

    /**
     * @param array<int, array{int, string, int}|string> $tokens
     */
    private function declaredName(array $tokens, int $index): ?string
    {
        $token = $tokens[$index];
        if (!is_array($token)) {
            return null;
        }

        if ($token[0] === T_VARIABLE) {
            $name = ltrim($token[1], '$');
            $previous = $this->neighbour($tokens, $index, -1);
            $isStaticAccess = is_array($previous) && $previous[0] === T_DOUBLE_COLON;

            return $isStaticAccess || in_array($name, self::IGNORED_VARIABLES, true) ? null : $name;
        }

        if ($token[0] === T_FUNCTION) {
            $previous = $this->neighbour($tokens, $index, -1);
            if (is_array($previous) && $previous[0] === T_USE) {
                return null;
            }

            $next = $this->neighbour($tokens, $index, 1);
            if ($next === '&') {
                $next = $this->neighbour($tokens, $this->neighbourIndex($tokens, $index, 1), 1);
            }

            return is_array($next) && $next[0] === T_STRING ? $next[1] : null;
        }

        return null;
    }

    /**
     * @param array<int, array{int, string, int}|string> $tokens
     *
     * @return array{int, string, int}|string|null
     */
    private function neighbour(array $tokens, ?int $index, int $direction)
    {
        $neighbourIndex = $this->neighbourIndex($tokens, $index, $direction);

        return $neighbourIndex === null ? null : $tokens[$neighbourIndex];
    }

    /**
     * @param array<int, array{int, string, int}|string> $tokens
     */
    private function neighbourIndex(array $tokens, ?int $index, int $direction): ?int
    {
        if ($index === null) {
            return null;
        }

        for ($i = $index + $direction; isset($tokens[$i]); $i += $direction) {
            $token = $tokens[$i];
            if (!is_array($token) || !in_array($token[0], [T_WHITESPACE, T_COMMENT, T_DOC_COMMENT], true)) {
                return $i;
            }
        }

        return null;
    }

    /**
     * @param array<int, array{int, string, int}|string> $tokens
     *
     * @return array<int, true>
     */
    private function findIgnoredLines(array $tokens): array
    {
        $lines = [];

        foreach ($tokens as $token) {
            if (is_array($token) && $token[0] === T_COMMENT && strpos($token[1], self::IGNORE_MARKER) !== false) {
                $lines[$token[2]] = true;
            }
        }

        return $lines;
    }
}
