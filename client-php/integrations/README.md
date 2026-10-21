# Integrations

The library needs no framework: everything here is a few lines or one file to copy. In every case
`dws_debugger.json` next to the project turns it on and off (see [the README](../README.md#start)),
and without it nothing happens. Errors of the pages (`jsTag()`) need [pushing to the
viewer](../README.md#pushing-to-the-viewer); without it the tag is empty.

Русская версия — [README.ru.md](README.ru.md).

## Plain PHP

```php
require __DIR__ . '/vendor/autoload.php';            // or require 'wapp-debugger.php', the one-file build
WappDebugger\Debugger::boot(__DIR__ . '/dws_debugger.json');
```

as the first lines of the front controller (`index.php`) and of each CLI entry point. Before
`</body>` of the layout:

```php
<?= WappDebugger\Debugger::instance()->jsTag() ?>
```

Queries: `WappDebugger\Sql::watch($pdo)` for PDO, `Debugger::instance()->sql()` for anything else.

## Laravel

1. Start the request in `public/index.php` and `artisan`, right after `vendor/autoload.php`:

   ```php
   WappDebugger\Debugger::boot(__DIR__ . '/../dws_debugger.json');   // artisan: __DIR__ . '/dws_debugger.json'
   ```

2. A log channel in `config/logging.php`, added to the stack (Laravel reports exceptions through the log,
   with their trace):

   ```php
   'stack' => ['driver' => 'stack', 'channels' => ['single', 'wapp-debugger']],
   'wapp-debugger' => ['driver' => 'custom', 'via' => WappDebugger\Bridge\LaravelChannel::class, 'level' => 'debug'],
   ```

3. Queries, in `AppServiceProvider::boot()`:

   ```php
   Illuminate\Support\Facades\DB::listen(static function ($query): void {
       WappDebugger\Debugger::instance()->sql($query->sql, $query->bindings, $query->time);
   });
   ```

4. The tag of the pages, before `</body>` of the layout: `{!! WappDebugger\Debugger::instance()->jsTag() !!}`.

`Http::` of Laravel is Guzzle: `Http::withMiddleware(WappDebugger\Bridge\GuzzleMiddleware::create())`.

## Symfony

1. Start the request in `public/index.php` and `bin/console`, after `vendor/autoload_runtime.php`:

   ```php
   WappDebugger\Debugger::boot(dirname(__DIR__) . '/dws_debugger.json');
   ```

2. Copy [`symfony/wapp_debugger.yaml`](symfony/wapp_debugger.yaml) to `config/packages/` (put it
   under `when@dev:` to keep it out of production). Every Monolog channel goes to the viewer;
   Doctrine logs its queries to the `doctrine` channel in dev.

3. The tag of the pages, before `</body>` of `base.html.twig`: `{{ wapp_debugger.jsTag()|raw }}`.

## WordPress

1. Copy [`wordpress/wapp-debugger.php`](wordpress/wapp-debugger.php) to `wp-content/mu-plugins/`.
2. Put the library next to it as `wapp-debugger-library.php` (the one-file build of a release), or
   install it with Composer in the site, or set `WAPP_DEBUGGER_LIBRARY` in `wp-config.php`.
3. Put `dws_debugger.json` into the site root (`ABSPATH`), or set `WAPP_DEBUGGER_CONFIG`.

Queries come with their time when `define('SAVEQUERIES', true);` is in `wp-config.php`. The tag of
the pages is printed in the footer of the site, of the admin and of the login page.

## 1C-Bitrix

1. Copy [`bitrix/wapp-debugger.php`](bitrix/wapp-debugger.php) to `/local/php_interface/`, the
   library next to it as `wapp-debugger-library.php` (or install it with Composer and require
   `vendor/autoload.php` first).
2. At the top of `/local/php_interface/init.php`: `require __DIR__ . '/wapp-debugger.php';`
3. Put `dws_debugger.json` into the document root, or set `WAPP_DEBUGGER_CONFIG`.

Queries of the D7 connection come with their time (the old `$DB` uses it too); the tag of the pages
is put before `</body>` of every page.
