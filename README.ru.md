# wapp_php_debugger

[English version](README.md)

Логгер-отладчик для PHP-проектов: PHP-библиотека пишет события каждого запроса в JSONL-логи, просмотрщик показывает запросы, сообщения, бэктрейсы и переменные в реальном времени.

> **Статус: ранняя версия.** Просмотрщик переписан на Go + Svelte, есть десктоп-версия на Wails. PHP-библиотека 2.x (PHP 7.4+, формат логов v2) готова в этом репозитории, но ещё не опубликована в Packagist; просмотрщик читает и логи библиотеки v1. План, фазы и решения — в [IMPROVEMENT_PLAN.md](IMPROVEMENT_PLAN.md), правила разработки — в [CONTRIBUTING.md](CONTRIBUTING.md) (на английском).

![wapp-debugger: лента запросов, запрос и сообщение с контекстом и бэктрейсом](docs/screenshot.png)

## Быстрый старт

1. **Соберите просмотрщик** (Go 1.26+, Node 22+):

   ```sh
   make ui-deps build                              # dist/wapp-debugger со встроенным UI
   ```

2. **Подключите библиотеку 2.x к проекту** (PHP 7.4+). Без Composer скопируйте в проект [`client-php/src`](client-php/src), затем как можно раньше в запросе:

   ```php
   require __DIR__ . '/lib/wapp-debugger/autoload.php'; // или vendor/autoload.php от Composer
   WappDebugger\Debugger::boot(__DIR__ . '/dws_debugger.json');
   ```

   и положите `dws_debugger.json` в корень проекта:

   ```json
   {
       "enabled": true,
       "logs_dir": "tmp/logs"
   }
   ```

   Папка логов должна быть доступна PHP на запись. Логирование из кода:

   ```php
   __d(DWS_INFO, 'Заказ создан', ['order' => $order]);
   $total = __log($price) * $quantity; // пишет значение прямо внутри выражения
   __start('db'); /* ... */ __stop('db'); // таймер
   ```

   Ошибки, исключения и фатальные ошибки пишутся сами, секреты маскируются. PSR-3, Monolog, все настройки и переход с библиотеки v1 — в [client-php/README.ru.md](client-php/README.ru.md). Старые проекты могут остаться на библиотеке v1 ([`client-php/v1/Debugger.class.php`](client-php/v1/Debugger.class.php)): просмотрщик читает обе.

3. **Запустите просмотрщик** на папке логов:

   ```sh
   dist/wapp-debugger serve --logs-dir /var/www/app/tmp/logs
   # откройте http://127.0.0.1:8820
   ```

   Корень проекта находится по `dws_debugger.json` выше папки логов. Новые и изменившиеся файлы логов импортируются в фоне, новые запросы появляются на странице сами. Другие проекты добавляются в UI («Добавить проект»); переключатель «Отладчик» там же меняет `bIsOnFlag`.

Интерфейс на английском и русском, темы — светлая, тёмная и высококонтрастная (переключатели в шапке). Поиск ищет слова в сообщениях и их контексте или часть URL.

## Десктоп-версия

```sh
make desktop-build                                 # в Linux нужны GTK4 + WebKitGTK 6.0
dist/wapp-debugger-desktop --logs-dir /var/www/app/tmp/logs
```

Тот же просмотрщик в окне, без сетевого порта.

## Docker

```sh
docker run -d --name wapp-debugger -e WAPP_DEBUGGER_TOKEN=<token> -p 127.0.0.1:8820:8820 \
  --user "$(id -u):$(id -g)" -v /var/www/app:/var/www/app -v wapp-debugger:/data \
  ghcr.io/hightemp/wapp-debugger --logs-dir /var/www/app/tmp/logs
# откройте http://127.0.0.1:8820/?token=<token>
```

В контейнере просмотрщик слушает все интерфейсы, поэтому токен обязателен. Монтируйте проект по тому же пути, что и на хосте: тогда пути в просмотрщике и ссылки в IDE совпадут. `--user` нужен, чтобы читать логи и переключать отладчик в `dws_debugger.json`. Локально образ собирается `make docker-build`.

## Доступ по сети

По умолчанию просмотрщик отвечает только на `127.0.0.1` и только на loopback-имена хоста — это заодно защищает от DNS rebinding. Чтобы открыть его другим машинам, задайте токен; без токена просмотрщик откажется слушать не-loopback адрес:

```sh
WAPP_DEBUGGER_TOKEN=$(openssl rand -hex 32) dist/wapp-debugger serve --addr 0.0.0.0:8820 --logs-dir …
# один раз откройте http://<host>:8820/?token=<token>: токен переедет в HttpOnly-cookie
# API-клиенты передают Authorization: Bearer <token>
```

Переменная окружения лучше `--token`: её не видно в списке процессов. Просмотрщик ставит строгий Content-Security-Policy, отклоняет запись с чужих origin и ограничивает тело запроса 1 МиБ. TLS он не терминирует: через недоверенные сети используйте TLS-прокси или SSH-туннель.

У самой библиотеки HTTP-команд по умолчанию нет (`?debug_msg=…` требует константу `DA_HTTP_TOKEN` и заголовок `X-Debugger-Token`); новый просмотрщик их не использует.

## Структура

| Каталог | Что |
|---|---|
| `server/` | Просмотрщик на Go: `wapp-debugger serve`, API, живые обновления, встроенный UI |
| `ui/` | Веб-интерфейс на Svelte, английский и русский |
| `server/cmd/wapp-debugger-desktop/` | Десктоп-версия на Wails v3 |
| `client-php/` | PHP-библиотека 2.x; в `v1/` — библиотека v1 для старых проектов |
| `spec/` | Форматы логов, контракты API и фикстуры |

Старого PHP-просмотрщика на EasyUI больше нет; он остался в истории git (тег `legacy-hotfix` — его последняя версия).

## Лицензия

[MIT](LICENSE)
