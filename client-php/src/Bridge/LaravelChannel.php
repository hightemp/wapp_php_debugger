<?php

declare(strict_types=1);

namespace WappDebugger\Bridge;

use Monolog\Logger;

/**
 * A custom channel of Laravel's config/logging.php (no Laravel class is needed):
 *     'wapp-debugger' => ['driver' => 'custom', 'via' => \WappDebugger\Bridge\LaravelChannel::class, 'level' => 'debug'],
 * then add 'wapp-debugger' to the channels of the stack channel.
 */
final class LaravelChannel
{
    /**
     * @param array<string, mixed> $config the channel config: level (default debug) and name
     */
    public function __invoke(array $config): Logger
    {
        $level = $config['level'] ?? 'debug';
        $name = $config['name'] ?? 'wapp-debugger';

        return new Logger(is_string($name) ? $name : 'wapp-debugger', [new MonologHandler(null, is_string($level) || is_int($level) ? $level : 'debug')]);
    }
}
