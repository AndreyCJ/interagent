# ADR-003: Storage — SQLite vs JSON files

**Date:** 2026-08-01
**Status:** Accepted
**Related documents:** 01-tz.md §7, 05-architecture.md §6

---

## Context

The app stores locally: session history (messages + metadata), agent configurations, and user settings. Requirements: no external services, macOS v1 (Intel + Apple Silicon), minimal setup. Data is not synced (see ADR-001).

## Options

### A. SQLite (via `modernc.org/sqlite`, pure Go, no cgo)

| Component | Technology                                                       |
| --------- | ---------------------------------------------------------------- |
| Storage   | SQLite via `modernc.org/sqlite` (or `mattn/go-sqlite3` with cgo) |

**Pros:**

- One DB file — easy to back up / migrate.
- Atomic transactions → no "half-saved" state (important for history).
- Fast even with thousands of messages (no loading everything into memory).
- More reliable than manual (JSON) serialization/deserialization.

**Cons:**

- A migration layer is needed (even if initially one file).
- `mattn/go-sqlite3` requires cgo → complicates CI builds on `macos-latest`. `modernc.org/sqlite` (pure Go) avoids this but is slightly slower.

### B. JSON files

| Component | Technology                                                              |
| --------- | ----------------------------------------------------------------------- |
| Storage   | `~/.config/interagent/` (or `~/Library/Application Support/Interagent`) |

**Pros:**

- No dependencies, readable by a human.
- Convenient for manual migrations (edit the file).

**Cons:**

- No atomic updates → risk of corrupting the file (crash mid-write).
- History file growth → the whole session in memory at startup.
- Harder to version the schema (manual migrations).

## Selection criteria

| Criterion   | A (SQLite)            | B (JSON)                     |
| ----------- | --------------------- | ---------------------------- |
| Reliability | ✅ (transactions)     | ⚠️ (atomicity)               |
| Performance | ✅                    | ❌ (whole session in memory) |
| cgo / build | ⚠️ (driver-dependent) | ✅                           |
| Migrations  | ⚠️ (layer needed)     | ⚠️ (manual)                  |
| Code size   | ⚠️                    | ✅ (less)                    |

## Decision

Option **A — SQLite** via `modernc.org/sqlite` (pure Go, no cgo) chosen for v1, so that Wails builds on CI are not broken.

> **Clarification (see ADR-005):** the project-wide rule is "pure Go where possible; cgo allowed only for whisper.cpp". Storage fully complies: `modernc.org/sqlite` — no cgo.

### Trade-offs

- A small migration layer is needed from the start (see `internal/adapter/storage`).
- As the number of sessions grows, the DB file can be compacted or archived manually.
