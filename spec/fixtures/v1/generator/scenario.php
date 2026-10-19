<?php
// Один сценарий = один запрос/процесс со старой библиотекой.
// CLI: php scenario.php <name> [args...]; веб: заголовок X-Fixture-Scenario (сервер php -S, см. web-driver.php).
// Каталог /var/www/app — как в реальных проектах, чтобы пути в бэктрейсах выглядели правдоподобно.

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
    // Процесс убит до shutdown-функций (как при OOM killer): заголовок остаётся с bFinished=false
    exec('kill -9 '.getmypid());
    sleep(5);
}

echo 'ok';
