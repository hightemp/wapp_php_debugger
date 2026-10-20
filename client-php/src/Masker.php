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

    /**
     * Hides the values of secret query parameters: https://pay.test/?id=7&token=*** — the rest of the
     * URL is kept as it is.
     */
    public function url(string $url): string
    {
        $parts = explode('?', $url, 2);
        if (count($parts) < 2) {
            return $url;
        }
        $hash = strpos($parts[1], '#');
        $query = $hash === false ? $parts[1] : substr($parts[1], 0, $hash);
        $fragment = $hash === false ? '' : substr($parts[1], $hash);
        $pairs = explode('&', $query);
        foreach ($pairs as $index => $pair) {
            [$key] = explode('=', $pair, 2);
            if ($key !== '' && $this->isSecret(rawurldecode($key))) {
                $pairs[$index] = $key . '=' . self::MASK;
            }
        }

        return $parts[0] . '?' . implode('&', $pairs) . $fragment;
    }
}
