# HANDOFF

Continuation notes for moving this work to another computer. Written 2026-08-26 on `stage-3`.
This file is temporary — likely deleted once the remaining items below are done.

## 1. Current state

- **Branch:** `stage-3` (published to `origin/stage-3`)
- **All tests green** at last run: `go test ./...`, `go vet ./...`, frontend `pnpm test` (64/64), lint, format, build.
- **`.env` is gitignored.** It holds `LLM_API_KEY=<real key>` locally and is NOT committed. `git pull` on the other machine will not bring it — recreate it there (see §4).

### Commit tree this session (top of `stage-3`, newest first)

```
8bf7e93 feat(audio): reliable SCK capture + transcription source for listening mode   [WIP carried over]
6848bfa chore: gitignore .env with API keys
9ff73a9 fix(storage): migrate default-local agent to default-cloud on startup
808d95f feat: clean up ChatPanel — remove AnswerBox, add auto-scroll and listening indicator
53773ce feat: add LLM_API_KEY env var support for cloud LLM
d5cd561 feat: change default agent from Ollama to OpenAI-compatible
9e90806 fix(session): bind SessionBind to current session and unify session instance   [earlier session]
... (earlier stage-3 history: audio pipelines, STT whisper, settings, etc.)
```

## 2. What this session built (listening mode + cloud LLM)

Goal: make the session chat fully functional — transcribed text appears live during listening, and
LLM answers stream in from an OpenAI-compatible cloud provider (DeepSeek by default).

- **d5cd561** — default agent seed changed from Ollama (`default-local`) to OpenAI-compatible
  (`default-cloud`, `deepseek-chat`, `https://api.deepseek.com`). Files: `internal/adapter/storage/storage.go`.
- **53773ce** — env-var override: `LLM_API_KEY` / `LLM_BASE_URL` / `LLM_MODEL`. If `LLM_API_KEY` is set,
  the factory builds an in-memory `env-llm` agent (plaintext key, never stored); otherwise stored agents
  are used and their encrypted key is decrypted. Clear error when no key: `"no API key configured — set LLM_API_KEY environment variable"`.
  Files: `app.go` (`envAgentConfig()` + factory), `internal/usecase/llm.go`, `app_env_test.go`.
- **808d95f** — `frontend/src/features/chat/ChatPanel.vue` rewritten: removed confusing `AnswerBox`
  ("Waiting" placeholder), auto-scroll, pulsing "Listening..." indicator, role-colored labels, streaming
  ellipsis for pending assistant reply, error line.
- **9ff73a9** — storage migration `migrateDefaultAgent()`: deletes legacy `default-local` and ensures
  `default-cloud` exists, so existing databases stop trying Ollama. Files: `internal/adapter/storage/storage.go` + test.
- **8bf7e93** — pre-existing uncommitted audio WIP committed for safekeeping (see §3).

**Why the chat was broken before:** the default agent pointed at Ollama (`http://localhost:11434`) which
isn't running, so every LLM call failed with `dial tcp [::1]:11434: connection refused`. The listening
backend pipeline itself (audio → whisper → `transcription:done` → `autoAnswer` → `llm:started/partial/response`)
was already wired and is untouched.

## 3. Carried-over WIP (commit 8bf7e93)

Uncommitted work from prior sessions, committed here so it survives the machine switch:

- `internal/adapter/audio/capture_darwin_objc.go` — ScreenCaptureKit reliability: include all apps in the
  SCContentFilter, real pixel dimensions (1x1 audio-only config fails with SCError 1003 on macOS 26),
  `config.sampleRate/channelCount/queueDepth`, richer error strings, tear down half-open streams on failed start.
- `internal/usecase/audiopipeline.go` + test — include `source` in `transcription:done` payload (already referenced the field).
- `scripts/dev-audio.sh` — signed-app dev loop (wails build → codesign → open). See ADR-012.
- `build/darwin/entitlements.plist`, `docs/adr/012-sc-content-filter-and-dev-signing.md`.
- Small frontend/audio test + `useAudio.ts` tweak, `frontend/vite.config.ts`, regenerated wailsjs runtime, `AGENTS.md`/docs tweaks.

## 4. PENDING: load `.env` in the Go app (NOT implemented yet)

**Symptom:** user added `.env` with `LLM_API_KEY=...` but still gets
`no API key configured — set LLM_API_KEY environment variable`.

**Root cause:** Go's `os.Getenv` reads real process env vars only. Nothing sources `.env` — neither
`wails dev` nor `wails build`. The app never sees the key.

**Fix — hand-rolled dotenv loader (no dependency):**

`app.go`, add alongside `envAgentConfig()`:

```go
// loadDotEnv reads KEY=VALUE pairs from .env files in well-known locations
// into the process environment. Real shell env vars always win.
func loadDotEnv() {
	for _, dir := range envFileDirs() {
		loadDotEnvFromDir(dir)
	}
}

// envFileDirs returns candidate dirs for a `.env` file: the working directory
// (wails dev from the repo root), the executable's dir, and the app config
// dir (so a signed build finds a config-placed .env too).
func envFileDirs() []string {
	dirs := []string{"."}
	if exe, err := os.Executable(); err == nil {
		dirs = append(dirs, filepath.Dir(exe))
	}
	if cfg, err := os.UserConfigDir(); err == nil {
		dirs = append(dirs, filepath.Join(cfg, "interagent"))
	}
	return dirs
}

func loadDotEnvFromDir(dir string) {
	data, err := os.ReadFile(filepath.Join(dir, ".env"))
	if err != nil {
		return
	}
	for key, value := range parseDotEnv(data) {
		if _, exists := os.LookupEnv(key); !exists {
			_ = os.Setenv(key, value)
		}
	}
}

// parseDotEnv parses dotenv content: skips blank lines and '#' comments,
// tolerates an optional "export " prefix, splits on the first '=', trims
// whitespace and strips surrounding single/double quotes.
func parseDotEnv(data []byte) map[string]string {
	env := map[string]string{}
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		line = strings.TrimSpace(strings.TrimPrefix(line, "export "))
		key, value, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		key = strings.TrimSpace(key)
		if key == "" {
			continue
		}
		env[key] = strings.Trim(strings.TrimSpace(value), `"'`)
	}
	return env
}
```

`main.go`, first line of `main()`:

```go
func main() {
	loadDotEnv()
	app := NewApp()
	...
```

`app.go` imports: add `"strings"` (`os` and `path/filepath` already imported).

Tests — `app_env_test.go`:

```go
package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestParseDotEnv_Basic(t *testing.T) {
	env := parseDotEnv([]byte("LLM_API_KEY=sk-test\nLLM_BASE_URL=https://api.example.com\n"))
	if env["LLM_API_KEY"] != "sk-test" {
		t.Errorf("LLM_API_KEY = %q, want sk-test", env["LLM_API_KEY"])
	}
	if env["LLM_BASE_URL"] != "https://api.example.com" {
		t.Errorf("LLM_BASE_URL = %q, want https://api.example.com", env["LLM_BASE_URL"])
	}
}

func TestParseDotEnv_Tolerances(t *testing.T) {
	env := parseDotEnv([]byte("export LLM_API_KEY=sk-1\n  LLM_MODEL = \"deepseek-chat\"  \n# comment\n\nNO_EQUALS\n"))
	if env["LLM_API_KEY"] != "sk-1" {
		t.Errorf("export prefix: %q, want sk-1", env["LLM_API_KEY"])
	}
	if env["LLM_MODEL"] != "deepseek-chat" {
		t.Errorf("quote trimming: %q, want deepseek-chat", env["LLM_MODEL"])
	}
	if _, ok := env["NO_EQUALS"]; ok {
		t.Error("line without '=' must be skipped")
	}
}

func TestLoadDotEnv_FillsUnsetVars_RealEnvWins(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, ".env"),
		[]byte("LLM_API_KEY=sk-file\nLLM_MODEL=model-file\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("LLM_API_KEY", "sk-env")
	loadDotEnvFromDir(dir)
	if got := os.Getenv("LLM_API_KEY"); got != "sk-env" {
		t.Errorf("real env should win: %q", got)
	}
	if got := os.Getenv("LLM_MODEL"); got != "model-file" {
		t.Errorf("unset var filled from file: %q", got)
	}
}
```

Verify: `go test ./... -run TestParseDotEnv` then `go test ./...`, then `LLM_API_KEY=... wails dev`.

**Trade-off considered:** `github.com/joho/godotenv` — rejected, adds a dependency for ~25 lines.

## 5. Recreate `.env` on the other machine

```bash
cat > .env <<'EOF'
LLM_API_KEY=<your deepseek key>
# optional overrides:
# LLM_BASE_URL=https://api.deepseek.com
# LLM_MODEL=deepseek-chat
EOF
```

Defaults when unset: base URL `https://api.deepseek.com`, model `deepseek-chat`.
The key in `.env` has been seen by the session; consider rotating it.

## 6. Decisions locked this session

- **Provider-agnostic:** nothing is hardcoded as "DeepSeek" beyond default values; any OpenAI-compatible
  provider works via env vars / agent settings (added later).
- **Mic behavior:** mic transcription stays history-only (`user` role); only system audio (interviewer)
  auto-triggers LLM answers (ADR-007 cancel-on-new-input). Do NOT change.

## 7. De-slop (docs) — done in this session

Docs mostly deleted; **code is the source of truth**:

- Deleted: `docs/00-documentation-map.md`, `01-tz.md`, `02-nfr.md`, `03-process.md`, `04-events.md`,
  `05-architecture.md`, `06-bind-contracts.md`, and `docs/superpowers/` (gitignored scratch).
- Kept: `docs/adr/` untouched; `AGENTS.md` rewritten (points contracts at `internal/port/*` and `bind_*.go`);
  `README.md` trimmed.
- `scripts/dev-audio.sh` still references `docs/adr/012-...` — valid, ADRs are kept.

## 8. Future work (the plan this session was titled over)

1. Implement §4 (`.env` loading).
2. Full listening-mode smoke test on real audio: Listen → transcription appears as `interviewer:` → LLM
   streams `assistant:`; manual text → `user:` → LLM; Stop stops cleanly.
3. Later, separately: model selection + API-key auth UI (env var is a stopgap), agent settings panel,
   partial/interim transcription (`onPartial` is wired in `audiopipeline.go` as `nil` today — see
   `frontend/src/features/overlay/TranscriptionBar.vue`, currently unused).
