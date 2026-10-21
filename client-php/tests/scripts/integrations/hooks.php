<?php

declare(strict_types=1);

// Hooks of WordPress as far as the mu-plugin uses them
$GLOBALS['wapp_test_hooks'] = [];

function add_filter(string $hook, callable $callback, int $priority = 10, int $acceptedArgs = 1): bool
{
    $GLOBALS['wapp_test_hooks'][$hook][] = [$callback, $acceptedArgs];

    return true;
}

function add_action(string $hook, callable $callback, int $priority = 10, int $acceptedArgs = 1): bool
{
    return add_filter($hook, $callback, $priority, $acceptedArgs);
}

/**
 * @param mixed ...$args
 */
function do_action(string $hook, ...$args): void
{
    foreach ($GLOBALS['wapp_test_hooks'][$hook] ?? [] as [$callback, $count]) {
        $callback(...array_slice($args, 0, $count));
    }
}

/**
 * @param mixed $value
 * @param mixed ...$args
 *
 * @return mixed
 */
function apply_filters(string $hook, $value, ...$args)
{
    foreach ($GLOBALS['wapp_test_hooks'][$hook] ?? [] as [$callback, $count]) {
        $value = $callback($value, ...array_slice($args, 0, $count - 1));
    }

    return $value;
}
