# Events (Backend → Frontend)

## Rules

1. **Events are the only async channel** backend → frontend. Bind methods returning `void` answer with an event.
2. **Data types and bind methods — in code, not here.** Source of truth for types:
   - Go: `internal/port/types.go`
   - TypeScript: `frontend/src/common/types/api.types.ts`
   - JS bindings are generated: `wails generate` → `frontend/wailsjs/`
3. **Event names and payloads — only here.** Wails events are strings with JSON data without type checking, so their contract is documented manually.
4. **Changing events — via PR + review** (see DoD in `03-process.md`).
5. **STT — local (whisper.cpp). LLM — local (Ollama, ADR-011) or cloud OpenAI-compatible, user's choice** (ADR-001, ADR-004).
