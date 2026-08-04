# NFR: Non-functional Requirements

**Date:** 2026-07-30
**Status:** Approved

---

All requirements are measured on the target configuration: MacBook Air M1, 8 GB RAM, macOS 14+. Inference is local (STT: whisper.cpp; LLM: Ollama (ADR-011)) or cloud (OpenAI-compatible), user's choice (see ADR-001, ADR-004, ADR-011).

## Requirements

- **NFR-01 — Response latency (Performance, High).** Time from the end of the interviewer's phrase to the hint appearing — no more than 5 s, target 2 s. Measured for local and cloud LLM (both ADR-001/ADR-004 modes). **Verification:** measure 10 iterations: stop-time after feeding test audio to overlay render.
- **NFR-02 — Confidentiality (Security, Critical).** Audio and screenshots do not leave the device **by default**. STT is local (whisper.cpp); audio never goes to the network. Only transcription/OCR text goes to the cloud LLM, plus **the screenshot as an image** — but only if (a) a cloud provider is enabled and (b) the "send screenshot as-is" path is selected (multimodal, ADR-005), always with an indicator and consent (ADR-004). Audio (microphone/system sound) never goes to the network. **Verification:** audit network calls through a proxy (mitmproxy / Charles): audio is always absent; the screenshot image leaves **only** with a cloud provider enabled + direct-image mode and with an indicator.
- **NFR-03 — CPU load (Performance, Medium).** Idle (overlay open, nothing processed) — no more than 5% CPU (one core). Local inference — no more than 80% for 10 s. Cloud processing — no more than 10%. **Verification:** `pidstat` / `top` in states: idle, local processing, cloud processing.
- **NFR-04 — Click-through overlay (UX, High).** The overlay window **by default** does not intercept mouse and keyboard events (click-through over the whole area). The interactive mode is toggled by a global shortcut (ADR-006) and in it the window can accept input (manual input, settings). **Verification:** test — click an arbitrary point of the overlay in click-through mode — the event goes to the window below (Xcode Accessibility Inspector / CGEvent); the shortcut toggle changes the mode.
- **NFR-05 — Global shortcuts (UX, High).** All app shortcuts work when the window is not focused, including when another app is active (in fullscreen). **Verification:** test — 10 shortcuts in a row from another app — all triggered.
- **NFR-06 — Graceful degradation (Reliability, Critical).** A crash or timeout of STT/LLM/OCR does not bring down the app. The user sees a message in the overlay: "Recognition / response error" with a retry option. **Verification:** inject errors into every adapter (network timeout, invalid auth, corrupted response) — the app does not crash.
- **NFR-07 — RAM usage (Performance, Medium).** Idle — no more than 200 MB RAM. While processing — no more than 400 MB (cloud) / local LLM — up to 4 GB with a warning to the user. **Verification:** `vmmap` / Activity Monitor in states: idle, cloud processing, local processing.
- **NFR-08 — Binary size (Compatibility, Low).** Installed app size without local models — no more than 50 MB. Models are downloaded separately. **Verification:** `du -sh` on the compiled .app.
- **NFR-09 — Error logging (Reliability, Medium).** All errors are written to a local log with timestamp, stage, context. The user can export the log with one button. No telemetry (opt-in). **Verification:** check the log file exists after every error scenario.
- **NFR-10 — Auto-update (UX, Low).** Updates are checked at startup. New version notification is unobtrusive (badge / toast). Background download with install confirmation. **Verification:** mock update: server returns a new version — the UI shows a notification.
- **NFR-11 — Key encryption (Security, Critical).** Cloud LLM provider API keys are stored in SQLite only encrypted (AES-256-GCM). The encryption master key — in macOS Keychain (service `com.interagent.keys`). **Verification:** check — no plaintext apiKey in the DB; after removing the Keychain key decryption is impossible; log export contains no keys.
