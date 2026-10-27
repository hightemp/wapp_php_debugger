# spec

**English** · [Русский](README.ru.md)

Shared contracts for the PHP library (`client-php/`), the Go server (`server/`) and the UI (`ui/`). Everything here is checked by the tests in `server/internal/spec`.

| Path | What |
|---|---|
| [`logformat/`](logformat/README.md) | Log formats v1 and v2: description and JSON Schema |
| [`fixtures/v1/`](fixtures/v1/README.md) | Real v1 logs and `manifest.json` with their reference properties |
| [`fixtures/v2/`](fixtures/v2/README.md) | Reference v2 logs, invalid events and invalid streams |
| [`openapi.yaml`](openapi.yaml) | Viewer API v1 |
| [`library-http.yaml`](library-http.yaml) | Optional HTTP mode of the PHP library 2.x |
| [`ingest.yaml`](ingest.yaml) | Receiver of pushed logs (`serve --ingest-addr`) |
| [`db/`](db/README.md) | Viewer database schema |
| [`frozen/`](frozen/README.md) | The contracts as they were at 1.0: later changes stay backward compatible |
