<?php
// One scenario = one request/process with the legacy library.
// CLI: php scenario.php <name> [args...]; web: X-Fixture-Scenario header (php -S server, see web-driver.php).
// /var/www/app as in real projects, so backtrace paths look realistic. The Cyrillic messages below are fixture data.

$scenario = PHP_SAPI === 'cli' ? $argv[1] : $_SERVER['HTTP_X_FIXTURE_SCENARIO'];

require getenv('FIXTURE_LIB') ?: __DIR__.'/Debugger.class.php';

__d(DWS_INFO, 'Сценарий '.$scenario, ['user' => ['id' => 7, 'name' => 'Тест', 'api_key' => 'should-be-masked']]);
__start('db');
__d(DWS_MESSAGE, 'SELECT * FROM items WHERE id = 42 /var/www/app/models/Item.php:15', ['rows' => [['id' => 42, 'price' => 9.5, 'active' => true, 'note' => null]]]);
__stop('db');
__log('inline-value');
__d(DWS_WARNING, 'Медленный ответ кеша', ['ms' => 120]);
trigger_error('Пользовательская ошибка', E_USER_ERROR);
fnDebugAddFoundField('price');
fnDebugAddNotFoundField('discount');

if ($scenario === 'killed') {
    // The process is killed before shutdown functions run (like the OOM killer): the header keeps bFinished=false
    exec('kill -9 '.getmypid());
    sleep(5);
}

echo 'ok';
