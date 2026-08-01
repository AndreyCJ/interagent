# Interagent

Приложение-оверлей для отображения подсказок в режиме реального времени.

**Конфиденциальность по умолчанию.** Аудио и скриншоты никогда не покидают устройство.
LLM может быть локальным (llama.go) или облачным (OpenAI-compatible) — выбор пользователя,
ApiKey хранится зашифрованным. См. [ADR-001](docs/adr/001-local-vs-api-llm.md).

## Как запустить

```
go generate ./...     # wails Generate
wails dev             # режим разработки (гарячее перезапуск фронта + бэкенд)
wails build           # production-сборка .app
```

Разработка требует: Go 1.25+, Node 22 + pnpm, macOS 14+.

## Документация

| Документ                                                     | Назначение                                                |
| ------------------------------------------------------------ | --------------------------------------------------------- |
| [docs/00-documentation-map.md](docs/00-documentation-map.md) | Карта документации, статусы, порядок чтения               |
| [docs/01-tz.md](docs/01-tz.md)                               | ТЗ, сущности, сценарии, словарь, этапы                    |
| [docs/02-nfr.md](docs/02-nfr.md)                             | Нефункциональные требования                               |
| [docs/03-process.md](docs/03-process.md)                     | Процесс: TDD, ревью, Definition of Done                   |
| [docs/04-events.md](docs/04-events.md)                       | Рантайм-события backend → frontend (типы/методы — в коде) |
| [docs/05-architecture.md](docs/05-architecture.md)           | Слои, пайплайн, окно, ошибки, хранилище                   |
| [docs/06-bind-contracts.md](docs/06-bind-contracts.md)       | Контракт bind-методов frontend ↔ backend                  |
| [docs/adr/](docs/adr/)                                       | Архитектурные решения (ADR-001…008)                       |

Агенты: см. [AGENTS.md](AGENTS.md).

## Проверки

```
go test ./... && go vet ./... && go fmt ./...     # Go
cd frontend && pnpm lint && pnpm format:check && pnpm test
cd frontend && pnpm exec playwright test           # e2e
```
