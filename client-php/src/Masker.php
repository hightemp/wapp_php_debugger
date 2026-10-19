<?php

declare(strict_types=1);

namespace WappDebugger;

/**
 * Decides which keys hold secrets. The default pattern is the one of library v1 (legacy-hotfix),
 * so the same values stay hidden after an upgrade.
 */
final class Masker
{
    public const MASK = '***';

    private const DEFAULT_PATTERN = '/(pass(wd|word)?$|^pwd$|secret|token|api_?key|app_?key|private_?key|access_?key|authori[sz]ation|php_auth_pw|cookie|dsn|csrf)/i';

    /** @var list<string> */
    private array $extraKeys;

    /**
     * @param list<string> $extraKeys more keys to hide, compared case-insensitively
     */
    public function __construct(array $extraKeys = [])
    {
        $this->extraKeys = array_map('strtolower', $extraKeys);
    }

    public function isSecret(string $key): bool
    {
        return preg_match(self::DEFAULT_PATTERN, str_replace('-', '_', $key)) === 1
            || in_array(strtolower($key), $this->extraKeys, true);
    }
}
