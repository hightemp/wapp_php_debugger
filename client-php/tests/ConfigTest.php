<?php

declare(strict_types=1);

namespace WappDebugger\Tests;

use PHPUnit\Framework\TestCase;
use WappDebugger\Config;
use WappDebugger\Tests\Support\TempDir;

final class ConfigTest extends TestCase
{
    use TempDir;

    public function testDefaults(): void
    {
        $config = Config::fromArray([]);

        self::assertFalse($config->enabled());
        self::assertSame(0664, $config->fileMode());
        self::assertSame(0775, $config->dirMode());
        self::assertSame([], $config->maskKeys());
        self::assertSame([6, 100, 2000], [$config->maxDepth(), $config->maxItems(), $config->maxStringBytes()]);
    }

    public function testLimitsAndMaskKeys(): void
    {
        $config = Config::fromArray(['mask_keys' => ['cvv', 7], 'max_depth' => 3, 'max_items' => 0, 'max_string_bytes' => 'x']);

        self::assertSame(['cvv'], $config->maskKeys(), 'only strings');
        self::assertSame(3, $config->maxDepth());
        self::assertSame(100, $config->maxItems(), 'invalid values fall back to the default');
        self::assertSame(2000, $config->maxStringBytes());
    }

    public function testMissingFileMeansDisabled(): void
    {
        self::assertFalse(Config::fromFile($this->tempDir() . '/dws_debugger.json')->enabled());
    }

    public function testReadsTheFileAndResolvesPathsAgainstIt(): void
    {
        $file = $this->tempDir() . '/dws_debugger.json';
        file_put_contents($file, '{"enabled": true, "logs_dir": "tmp/logs"}');

        $config = Config::fromFile($file);

        self::assertTrue($config->enabled());
        self::assertSame($this->tempDir(), $config->projectRoot(), 'the root defaults to the directory of the config');
        self::assertSame($this->tempDir() . '/tmp/logs', $config->logsDir());
    }

    public function testBrokenFileMeansDisabled(): void
    {
        $file = $this->tempDir() . '/dws_debugger.json';
        file_put_contents($file, '{"enabled": tru');

        self::assertFalse(Config::fromFile($file)->enabled());
    }

    public function testOldKeysOfLibraryV1(): void
    {
        $config = Config::fromArray([
            'bIsOnFlag' => true, 'sProjectRootPath' => '/var/www/app', 'sMessagesFilesPath' => '/var/www/app/tmp/logs',
            'sTempPath' => '/var/www/app/tmp', 'iArraySizeLimit' => 50, 'bLogSessionAndCookies' => true,
        ]);

        self::assertTrue($config->enabled());
        self::assertSame('/var/www/app', $config->projectRoot());
        self::assertSame('/var/www/app/tmp/logs', $config->logsDir());
        self::assertSame(50, $config->maxItems());
        self::assertSame(['get', 'post', 'files', 'session', 'cookie'], $config->logGlobals());
        self::assertSame(
            ['bIsOnFlag' => 'enabled', 'sProjectRootPath' => 'project_root', 'sMessagesFilesPath' => 'logs_dir', 'sTempPath' => null,
                'iArraySizeLimit' => 'max_items', 'bLogSessionAndCookies' => 'log_globals'],
            $config->oldKeys()
        );
    }

    public function testNewKeysWinOverOldOnes(): void
    {
        $config = Config::fromArray(['bIsOnFlag' => true, 'enabled' => false, 'logs_dir' => '/new', 'sMessagesFilesPath' => '/old']);

        self::assertFalse($config->enabled());
        self::assertSame('/new', $config->logsDir());
    }

    public function testGlobalsToLog(): void
    {
        self::assertSame(['get', 'post', 'files'], Config::fromArray([])->logGlobals(), '$_SERVER, session and cookies only on request');
        self::assertSame(['server', 'get'], Config::fromArray(['log_globals' => ['server', 'GET', 'bogus']])->logGlobals());
    }

    public function testModesAsOctalStringsInJson(): void
    {
        $file = $this->tempDir() . '/dws_debugger.json';
        file_put_contents($file, '{"file_mode": "0640", "dir_mode": "750"}');
        $config = Config::fromFile($file);

        self::assertSame(0640, $config->fileMode(), 'JSON has no octal numbers');
        self::assertSame(0750, $config->dirMode());
        self::assertSame(0664, Config::fromArray(['file_mode' => '0999'])->fileMode(), 'not octal: the default');
    }

    protected function tearDown(): void
    {
        putenv('APP_ENV');
        putenv('STAGE');
        unset($_SERVER['APP_ENV'], $_ENV['APP_ENV']);
    }

    /**
     * @param array<string, mixed> $values
     *
     * @return array<string, mixed>
     */
    private static function enabledWith(array $values): array
    {
        return ['enabled' => true, 'logs_dir' => '/tmp/logs', 'http_token' => str_repeat('t', 32)] + $values;
    }

    public function testPushNeedsAnHttpAddressAndALongToken(): void
    {
        $token = str_repeat('p', 32);
        $config = Config::fromArray(self::enabledWith(['push_url' => 'http://viewer.lan:8821/ingest/v2', 'push_token' => $token]));
        self::assertSame('http://viewer.lan:8821/ingest/v2', $config->pushUrl());
        self::assertSame($token, $config->pushToken());

        foreach ([
            'no token' => ['push_url' => 'http://viewer.lan:8821/ingest/v2'],
            'a short token' => ['push_url' => 'http://viewer.lan:8821/ingest/v2', 'push_token' => 'short'],
            'not http' => ['push_url' => 'file:///etc/passwd', 'push_token' => $token],
            'not a string' => ['push_url' => 8821, 'push_token' => $token],
        ] as $case => $values) {
            self::assertNull(Config::fromArray(self::enabledWith($values))->pushUrl(), $case);
        }
    }

    public function testPushIsOffOutsideAllowedEnvs(): void
    {
        putenv('APP_ENV=production');
        $config = Config::fromArray(self::enabledWith(['allowed_envs' => ['dev'], 'push_url' => 'https://viewer.lan/ingest/v2', 'push_token' => str_repeat('p', 32)]));

        self::assertNull($config->pushUrl());
    }

    public function testWithoutAllowedEnvsAnyEnvironmentWorks(): void
    {
        putenv('APP_ENV=production');

        self::assertTrue(Config::fromArray(self::enabledWith([]))->enabled());
    }

    public function testAllowedEnvsLetTheDebuggerWorkOnlyThere(): void
    {
        $values = self::enabledWith(['allowed_envs' => ['local', 'dev', 'stage']]);

        putenv('APP_ENV=dev');
        $config = Config::fromArray($values);
        self::assertTrue($config->enabled());
        self::assertNotNull($config->httpToken());

        putenv('APP_ENV=production');
        $config = Config::fromArray($values);
        self::assertFalse($config->enabled(), 'a production server stays silent even with enabled: true');
        self::assertNull($config->httpToken(), 'and the HTTP mode cannot turn it on');
    }

    public function testAnUnknownEnvironmentIsNotAllowed(): void
    {
        $config = Config::fromArray(self::enabledWith(['allowed_envs' => ['dev']]));

        self::assertFalse($config->enabled(), 'no APP_ENV at all: off, the safe side');
    }

    public function testTheEnvironmentComesFromServerAndEnvToo(): void
    {
        $values = self::enabledWith(['allowed_envs' => ['dev']]);

        $_SERVER['APP_ENV'] = 'dev'; // SetEnv of Apache, fastcgi_param of nginx
        self::assertTrue(Config::fromArray($values)->enabled());
        unset($_SERVER['APP_ENV']);

        $_ENV['APP_ENV'] = 'dev';
        self::assertTrue(Config::fromArray($values)->enabled());
    }

    public function testTheVariableNameIsConfigurable(): void
    {
        putenv('APP_ENV=dev');
        putenv('STAGE=prod');

        self::assertFalse(Config::fromArray(self::enabledWith(['allowed_envs' => ['dev'], 'env_var' => 'STAGE']))->enabled());
    }

    public function testAllowedEnvsIgnoreCaseAndJunk(): void
    {
        putenv('APP_ENV=Dev');

        self::assertTrue(Config::fromArray(self::enabledWith(['allowed_envs' => ['DEV', 7, null]]))->enabled());
        self::assertTrue(Config::fromArray(self::enabledWith(['allowed_envs' => 'dev']))->enabled(), 'a string is one environment');
        self::assertFalse(Config::fromArray(self::enabledWith(['allowed_envs' => 'prod']))->enabled());
    }
}
