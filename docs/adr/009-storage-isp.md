# ADR-009: Narrow storage interfaces (ISP)

**Date:** 2026-08-01
**Status:** ✅ Accepted
**Related documents:** 05-architecture.md (ports table), 003-storage.md, 00-documentation-map.md, `internal/port/storage.go`

---

## Context

`port.Storage` was one monolithic interface: CRUD for Session, Agents, and Settings at once. Because of this, every usecase (Session, Agent, Settings) formally depended on all three storage domains, and test mocks were forced to implement 9 methods of which only 2–3 were actually used. The trigger was a Stage-1 check: in the contract-test review the question arose "why does the agents mock implement CreateSession/GetSettings" — the answer: the monolithic interface forces it.

## Options

### A. Monolithic `Storage`

Pros: one interface, minimal types.
Cons: the usecase depends on extra stuff; mocks bloat with no-op methods; violates the interface segregation principle (ISP) and the "clean dependencies" principle (05-architecture).

### B. Three narrow interfaces

`SessionStorage`, `AgentStorage`, `SettingsStorage`.

Pros: the usecase depends only on what it needs; mocks are minimal (2–3 methods); the real adapter (SQLite) implements all three at no extra cost.
Cons: more types; if cross-domain operations appear (e.g., "delete everything for a session"), interface composition will be needed.

## Selection criteria

| Criterion                    | A (monolith) | B (narrow) |
| ---------------------------- | ------------ | ---------- |
| ISP / dependency cleanliness | ❌           | ✅         |
| Test mock simplicity         | ⚠️           | ✅         |
| Number of types              | ✅           | ⚠️         |
| SQLite adapter               | ✅           | ✅         |

## Decision

Option **B** chosen — three narrow interfaces `SessionStorage`, `AgentStorage`, `SettingsStorage` in `internal/port/storage.go`. Usecase constructors accept only the needed interface.

## Trade-offs

- Three interfaces instead of one; the storage adapter implements all three (composition in a single type).
- Storage technology is unchanged (SQLite, ADR-003).
- If an operation working across several domains appears later — interface composition will be added (without changing existing ones).
