<?php

declare(strict_types=1);

namespace WappDebugger\Bridge;

use WappDebugger\Debugger;

/**
 * Guzzle middleware that logs every request with its status or error and its duration:
 * $stack->push(GuzzleMiddleware::create()). Written against the shape of Guzzle (a handler that
 * returns a promise with then()), not its classes, so the library needs no Guzzle to load.
 */
final class GuzzleMiddleware
{
    /**
     * @return callable(callable): callable
     */
    public static function create(): callable
    {
        return static function (callable $handler): callable {
            return static function ($request, array $options) use ($handler) {
                $debugger = Debugger::instance();
                $started = $debugger->monotonicMs();
                $method = (string) $request->getMethod();
                $url = (string) $request->getUri();

                return $handler($request, $options)->then(
                    static function ($response) use ($debugger, $started, $method, $url) {
                        $debugger->http($method, $url, (int) $response->getStatusCode(), $debugger->monotonicMs() - $started);

                        return $response;
                    },
                    static function ($reason) use ($debugger, $started, $method, $url) {
                        $message = $reason instanceof \Throwable ? $reason->getMessage() : (is_scalar($reason) ? (string) $reason : 'request failed');
                        $debugger->http($method, $url, null, $debugger->monotonicMs() - $started, $message);

                        // A thrown reason keeps the promise rejected
                        throw $reason instanceof \Throwable ? $reason : new \RuntimeException($message);
                    }
                );
            };
        };
    }
}
