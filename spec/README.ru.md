# spec

[English](README.md) · **Русский**

Общие контракты PHP-библиотеки (`client-php/`), Go-сервера (`server/`) и UI (`ui/`). Всё здесь проверяется тестами в `server/internal/spec`.

| Путь | Что |
|---|---|
| [`logformat/`](logformat/README.ru.md) | Форматы логов v1 и v2: описание и JSON Schema |
| [`fixtures/v1/`](fixtures/v1/README.ru.md) | Настоящие логи v1 и `manifest.json` с их эталонными свойствами |
| [`fixtures/v2/`](fixtures/v2/README.ru.md) | Эталонные логи v2, неверные события и неверные потоки |
| [`openapi.yaml`](openapi.yaml) | API просмотрщика v1 |
| [`library-http.yaml`](library-http.yaml) | Необязательный HTTP-режим PHP-библиотеки 2.x |
| [`ingest.yaml`](ingest.yaml) | Приёмник отправленных логов (`serve --ingest-addr`) |
| [`db/`](db/README.ru.md) | Схема базы данных просмотрщика |
| [`frozen/`](frozen/README.ru.md) | Контракты в том виде, в каком они были в 1.0: дальнейшие изменения остаются обратно совместимыми |
