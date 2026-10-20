<?php

declare(strict_types=1);

namespace WappDebugger\Sql;

use WappDebugger\Debugger;

/**
 * A PDO statement that times execute() and logs the query with its parameters (Sql::watch()).
 * Signatures stay untyped so the class fits PHP 7.4 and 8.x alike.
 */
class LoggingStatement extends \PDOStatement
{
    /** @var array<array-key, mixed> values bound with bindValue() and bindParam() */
    private array $bound = [];

    /** PDO makes the statements itself: a public constructor is not allowed. */
    protected function __construct()
    {
    }

    /**
     * @param int|string $param
     * @param mixed      $value
     * @param int        $type
     */
    #[\ReturnTypeWillChange]
    public function bindValue($param, $value, $type = \PDO::PARAM_STR)
    {
        $this->bound[$param] = $value;

        return parent::bindValue($param, $value, $type);
    }

    /**
     * @param int|string $param
     * @param mixed      $var
     * @param int        $type
     * @param int|null   $maxLength
     * @param mixed      $driverOptions
     */
    #[\ReturnTypeWillChange]
    public function bindParam($param, &$var, $type = \PDO::PARAM_STR, $maxLength = null, $driverOptions = null)
    {
        $this->bound[$param] = &$var;

        return $maxLength === null ? parent::bindParam($param, $var, $type) : parent::bindParam($param, $var, $type, $maxLength, $driverOptions);
    }

    /**
     * @param array<array-key, mixed>|null $params
     */
    #[\ReturnTypeWillChange]
    public function execute($params = null)
    {
        $debugger = Debugger::instance();
        $started = $debugger->monotonicMs();
        $values = $params ?? $this->bound;
        try {
            $result = $params === null ? parent::execute() : parent::execute($params);
        } catch (\Throwable $error) {
            $debugger->log('error', $this->queryString, ['params' => $values, 'error' => $error->getMessage()], ['sql']);

            throw $error;
        }
        $duration = $debugger->monotonicMs() - $started;
        if ($result) {
            $debugger->sql($this->queryString, $values, $duration, $this->rowCount());
        } else {
            $info = $this->errorInfo();
            $debugger->log('error', $this->queryString, ['params' => $values, 'error' => (string) ($info[2] ?? 'failed')], ['sql']);
        }

        return $result;
    }
}
