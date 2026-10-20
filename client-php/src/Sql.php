<?php

declare(strict_types=1);

namespace WappDebugger;

/**
 * Logs the SQL of a PDO connection without changes to the queries: every prepared statement times
 * its execute() and writes it with Debugger::sql(). PDO::query() and PDO::exec() run without a
 * statement of ours, so they are not seen: prepare them, or log with Debugger::sql() by hand.
 */
final class Sql
{
    /**
     * Makes the statements of $pdo log themselves into the current request. A statement class the
     * application set before is replaced.
     */
    public static function watch(\PDO $pdo): \PDO
    {
        $pdo->setAttribute(\PDO::ATTR_STATEMENT_CLASS, [Sql\LoggingStatement::class, []]);

        return $pdo;
    }
}
