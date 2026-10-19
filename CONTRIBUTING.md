# Как вносить изменения

План и все принятые решения — в [IMPROVEMENT_PLAN.md](IMPROVEMENT_PLAN.md). Порядок работ — п. 8, решения — п. 10.

## Структура репозитория

| Каталог | Что | Язык |
|---|---|---|
| `client-php/` | PHP-библиотека (отладчик), Composer-пакет, PHP 7.4+ | PHP |
| `server/` | Просмотрщик: CLI, сервер, импорт логов | Go 1.26+ |
| `ui/` | Веб-интерфейс, встраивается в Go-бинарник | Svelte 5 + TypeScript |
| `desktop/` | Десктоп-версия на Wails (с фазы 4) | Go |
| `spec/` | Контракты: фикстуры, JSON Schema, OpenAPI | — |
| `legacy/` | Старый сервер, EasyUI и библиотека v1. Только срочные исправления, удаляется в фазе 5 | PHP, JS |

## Команды

Все проверки запускаются через `make`, CI вызывает те же цели:

```sh
make doctor   # что установлено, чего не хватает
make tools    # линтеры закреплённых версий в .tools/bin
make deps     # зависимости всех компонентов
make test     # все тесты
make lint     # все линтеры
make ci       # всё, что делает CI
make help     # остальные цели
```

## TDD

Новый код пишется через тесты:

1. Тест, который описывает нужное поведение, — и он падает (красный).
2. Минимальная реализация, при которой тест проходит (зелёный).
3. Рефакторинг при зелёных тестах.

Код без теста, который его потребовал, не принимается. Ошибка, найденная вручную, сначала воспроизводится тестом, потом исправляется.

| Компонент | Тесты | Где |
|---|---|---|
| `client-php` | PHPUnit 9.6, матрица PHP 7.4–8.4 в CI | `client-php/tests/` |
| `server` | `go test -race` | рядом с кодом, `*_test.go` |
| `ui` | Vitest + Testing Library | рядом с кодом, `*.test.ts` |

Тесты на разбор логов используют фикстуры из `spec/fixtures/`, а не придуманные строки.

## Контракты и сгенерированный код

Сначала меняется контракт в `spec/` (формат логов, OpenAPI), потом код. Типы Go (`server/internal/api`, `server/internal/libclient`) и TypeScript (`ui/src/lib/api/schema.gen.d.ts`) генерируются из OpenAPI командой `make generate`; файлы `*.gen.*` руками не правятся. CI проверяет, что сгенерированный код совпадает с закоммиченным (`make generate-check`).

## Именование

Венгерская нотация запрещена во всём новом коде (п. 4.4 плана): никаких префиксов типа `a`, `s`, `i`, `b`, `o`, `m`, `f`, `fn` (`$aVars`, `$sMessage`, `fnDebugLog`, `oSelectedProject`). Тип выражается системой типов.

| Где | Стиль | Пример |
|---|---|---|
| PHP | PSR-12: классы `PascalCase`, методы и свойства `camelCase`, константы `UPPER_SNAKE` | `$projectRoot`, `log()`, `Level::ERROR` |
| Go | идиоматичный Go (`MixedCaps`) | `Project.LogDir()` |
| TypeScript / Svelte | `camelCase`, компоненты `PascalCase` | `selectedRequest`, `RequestList.svelte` |
| Формат логов, конфиг, API, БД | `snake_case` | `request_id`, `is_cli`, `messages.level` |

Поля DTO из OpenAPI в TypeScript остаются `snake_case` (без слоя конвертации).

Проверяется автоматически: `client-php/tools/bin/check-naming` для PHP, ESLint `naming-convention` для TS/Svelte. Для deprecated-обёрток со старыми именами (п. 10.23) строку можно исключить комментарием `// naming-check:ignore`.

Словарь: **запрос** (request) — один HTTP- или CLI-запуск и его файл лога; **сообщение** (message) — событие внутри запроса; **контекст** (context) — переменные сообщения; **сервер** — машина, где лежат проекты; **проект** — приложение на сервере.

## Коммиты

[Conventional Commits](https://www.conventionalcommits.org/), описание на русском:

```
feat(server): импорт логов формата v1
fix(client-php): маскировать секреты во вложенных массивах
chore: обновить golangci-lint до v2.15
```

Типы: `feat`, `fix`, `refactor`, `test`, `docs`, `chore`, `ci`, `build`. Область — каталог верхнего уровня (`client-php`, `server`, `ui`, `desktop`, `spec`, `legacy`).
