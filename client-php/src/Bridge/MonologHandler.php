<?php

declare(strict_types=1);

namespace WappDebugger\Bridge;

/*
 * MonologHandler is the handler for the installed Monolog: Monolog 3 passes a LogRecord to write(),
 * Monolog 2 an array, and one class cannot declare both. Usage:
 *     $logger->pushHandler(new \WappDebugger\Bridge\MonologHandler());
 */
if (class_exists(\Monolog\LogRecord::class)) {
    class_alias(Monolog3Handler::class, MonologHandler::class);
} else {
    class_alias(Monolog2Handler::class, MonologHandler::class);
}
