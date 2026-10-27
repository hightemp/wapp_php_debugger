# Интеграции

[English](README.md) · **Русский**

Библиотеке не нужен фреймворк: всё здесь — несколько строк или один файл для копирования. Везде
включает и выключает отладчик `dws_debugger.json` рядом с проектом (см. [README](../README.ru.md#запуск)),
без него ничего не происходит. Ошибкам страниц (`jsTag()`) нужна [отправка в
просмотрщик](../README.ru.md#отправка-в-просмотрщик); без неё тег пустой.

## Обычный PHP

```php
require __DIR__ . '/vendor/autoload.php';            // или require 'wapp-debugger.php', однофайловая сборка
WappDebugger\Debugger::boot(__DIR__ . '/dws_debugger.json');
```

первыми строками фронт-контроллера (`index.php`) и каждой точки входа CLI. Перед `</body>` шаблона:

```php
<?= WappDebugger\Debugger::instance()->jsTag() ?>
```

Запросы к БД: `WappDebugger\Sql::watch($pdo)` для PDO, `Debugger::instance()->sql()` для всего остального.

## Laravel

1. Начать запрос в `public/index.php` и `artisan` сразу после `vendor/autoload.php`:

   ```php
   WappDebugger\Debugger::boot(__DIR__ . '/../dws_debugger.json');   // artisan: __DIR__ . '/dws_debugger.json'
   ```

2. Канал логов в `config/logging.php`, добавленный в stack (исключения Laravel сообщает через лог,
   с их трейсом):

   ```php
   'stack' => ['driver' => 'stack', 'channels' => ['single', 'wapp-debugger']],
   'wapp-debugger' => ['driver' => 'custom', 'via' => WappDebugger\Bridge\LaravelChannel::class, 'level' => 'debug'],
   ```

3. Запросы к БД — в `AppServiceProvider::boot()`:

   ```php
   Illuminate\Support\Facades\DB::listen(static function ($query): void {
       WappDebugger\Debugger::instance()->sql($query->sql, $query->bindings, $query->time);
   });
   ```

4. Тег страниц перед `</body>` шаблона: `{!! WappDebugger\Debugger::instance()->jsTag() !!}`.

`Http::` в Laravel — это Guzzle: `Http::withMiddleware(WappDebugger\Bridge\GuzzleMiddleware::create())`.

## Symfony

1. Начать запрос в `public/index.php` и `bin/console` после `vendor/autoload_runtime.php`:

   ```php
   WappDebugger\Debugger::boot(dirname(__DIR__) . '/dws_debugger.json');
   ```

2. Скопировать [`symfony/wapp_debugger.yaml`](symfony/wapp_debugger.yaml) в `config/packages/`
   (под `when@dev:`, чтобы не попал в production). Все каналы Monolog идут в просмотрщик; Doctrine
   в dev пишет свои запросы в канал `doctrine`.

3. Тег страниц перед `</body>` в `base.html.twig`: `{{ wapp_debugger.jsTag()|raw }}`.

## WordPress

1. Скопировать [`wordpress/wapp-debugger.php`](wordpress/wapp-debugger.php) в `wp-content/mu-plugins/`.
2. Положить библиотеку рядом как `wapp-debugger-library.php` (однофайловая сборка из релиза), или
   установить её Composer-ом в сайт, или задать `WAPP_DEBUGGER_LIBRARY` в `wp-config.php`.
3. Положить `dws_debugger.json` в корень сайта (`ABSPATH`) или задать `WAPP_DEBUGGER_CONFIG`.

Запросы к БД приходят с длительностью, когда в `wp-config.php` есть `define('SAVEQUERIES', true);`.
Тег страниц выводится в подвале сайта, админки и страницы входа.

## 1С-Битрикс

1. Скопировать [`bitrix/wapp-debugger.php`](bitrix/wapp-debugger.php) в `/local/php_interface/`,
   библиотеку рядом как `wapp-debugger-library.php` (или установить Composer-ом и подключить
   `vendor/autoload.php` раньше).
2. В начале `/local/php_interface/init.php`: `require __DIR__ . '/wapp-debugger.php';`
3. Положить `dws_debugger.json` в корень сайта или задать `WAPP_DEBUGGER_CONFIG`.

Запросы соединения D7 приходят с длительностью (старый `$DB` работает через него же); тег страниц
вставляется перед `</body>` каждой страницы.
