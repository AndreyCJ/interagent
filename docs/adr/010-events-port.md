# ADR-010: Events port (Event Bus)

**Date:** 2026-08-02
**Status:** Accepted
**Related documents:** 04-events.md, 05-architecture.md, 06-bind-contracts.md (Stage 2)

---

## Context

The backend reports async results to the frontend (LLM response, overlay mode change, permission statuses — see `04-events.md`) only via events. In stage 1 the usecases had no async operations: `LLM.SendText` synchronously returned an error, and bind methods returned only synchronous results.

Starting with stage 2 (`LLM.SendText` generates a response asynchronously and sends `llm:response`, overlay mode toggling and permission statuses), usecases must initiate events. To avoid tying `usecase` to Wails (violating clean dependencies: `adapter → port ← usecase`), events are initiated through a port.

## Options

### A. `Events` port in `internal/port` [chosen]

```go
// events.go
type Events interface {
    Emit(name string, payload any) error
}
```

- `usecase` depends on `port.Events` (an interface); adapters/app composition provide a Wails `runtime.EventsEmit` implementation.
- Testability: usecases are tested with a `port.Events` mock, checking which events are emitted and with what payload.

**Pros:** clean dependencies, unit tests cover events, transport replacement (Wails → test/future runtime) without touching usecases.
**Cons:** one more port/interface.

### B. Hard-tie usecase to the Wails runtime via a wrapper function in `bind`

The usecase receives `func(name string, payload any)` — a wrapper without a port.

**Pros:** fewer abstractions.
**Cons:** implicit contract, harder to mock, scattered transport layer — does not match the layered model (`bind → usecase → port`).

## Selection criteria

| Criterion              | A (port) | B (function) |
| ---------------------- | -------- | ------------ |
| Layer cleanliness      | ✅       | ⚠️           |
| Event testability      | ✅       | ⚠️           |
| Explicit type contract | ✅       | ❌           |

## Decision

Option **A** chosen — the `Events` port in `internal/port/events.go`. The Wails implementation lives in `adapter/events` (a wrapper over `runtime.EventsEmit`). The `overlay`, `llm`, `permissions`, `hotkeys` usecases use it to emit events from `04-events.md`.

## Trade-offs

- `Emit` is best used for short payloads (string, struct, serializable to JSON `wails runtime`): Wails events are JSON. Heavy data is better passed via separate bind calls.
- The `adapter/events` implementation is not covered by unit tests (a wrapper over runtime); it is tested by e2e/manual checks.
