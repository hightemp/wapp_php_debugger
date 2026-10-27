# Фикстуры логов v2

[English](README.md) · **Русский**

Написаны вручную по спецификации [`../../logformat/README.ru.md`](../../logformat/README.ru.md) ещё до библиотеки 2.x: это эталон и для писателя (`client-php`), и для парсера (`server`).

| Файл | Что показывает |
|---|---|
| `web-request.jsonl` | GET: все уровни, теги, контекст с замаскированным ключом, `source` и `trace`, таймер, `php_error` |
| `ajax-child-request.jsonl` | AJAX POST с `parent_request_id`, указывающим на `web-request`, замаскированный пароль |
| `cli-exception.jsonl` | CLI: медленный таймер, исключение с `previous`, фатальная ошибка, `exit_code` |
| `sql-and-http.jsonl` | SQL-запросы (один повторяется: N+1) и исходящий HTTP-вызов как сообщения с `duration_ms` |
| `js-page.jsonl` | Что пишет `debugger.js` для страницы `web-request`: непойманный `TypeError` со стеком, необработанный rejection, `console.warn` |
| `killed-without-end.jsonl` | Нет `request_end`: процесс убит или ещё работает |
| `invalid/*.json` | По одному событию, которое схема должна отклонить; имя файла говорит почему |
| `invalid-streams/*.jsonl` | Валидные события, которые нарушают правила потока |

Часть сообщений на русском намеренно: фикстуры проверяют не-ASCII текст.
