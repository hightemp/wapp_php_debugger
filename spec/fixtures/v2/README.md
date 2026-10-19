# Фикстуры логов v2

Написаны вручную по спецификации [`../../logformat/README.md`](../../logformat/README.md) до появления библиотеки 2.x: это эталон, под который будут писаться и писатель (`client-php`), и парсер (`server`).

| Файл | Что показывает |
|---|---|
| `web-request.jsonl` | GET: все уровни, теги, контекст с замаскированным ключом, `source` и `trace`, таймер, `php_error` |
| `ajax-child-request.jsonl` | AJAX POST с `parent_request_id` на `web-request`, замаскированный пароль |
| `cli-exception.jsonl` | CLI: медленный таймер, исключение с `previous`, фатальная ошибка, `exit_code` |
| `killed-without-end.jsonl` | Нет `request_end`: процесс убит или ещё работает |
| `invalid/*.json` | По одному событию, которое схема обязана отклонить; причина — в имени файла |
| `invalid-streams/*.jsonl` | Корректные события, нарушающие правила потока |
