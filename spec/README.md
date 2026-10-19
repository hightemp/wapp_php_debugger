# spec

Общие контракты для PHP-библиотеки (`client-php/`), Go-сервера (`server/`) и UI (`ui/`). Всё здесь проверяется тестами `server/internal/spec`.

| Путь | Что |
|---|---|
| [`logformat/`](logformat/README.md) | Форматы логов v1 и v2: описание и JSON Schema |
| [`fixtures/v1/`](fixtures/v1/README.md) | Настоящие логи v1 и `manifest.json` с их эталонными свойствами |
| [`fixtures/v2/`](fixtures/v2/README.md) | Эталонные логи v2, неверные события и потоки |
| [`openapi.yaml`](openapi.yaml) | API просмотрщика v1 |
| [`library-http.yaml`](library-http.yaml) | Опциональный HTTP-режим PHP-библиотеки 2.x |
| [`db/`](db/README.md) | Схема БД просмотрщика |
