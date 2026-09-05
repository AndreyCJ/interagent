# ADR-012: System sound capture — SCContentFilter scope and dev code signing

**Date:** 2026-08-07
**Status:** Accepted
**Related documents:** ADR-007 (audio pipeline), ADR-008 (macOS permissions), 05-architecture.md §3, 02-nfr.md (NFR-06)

---

## Context

System sound capture (ADR-007) uses a ScreenCaptureKit `SCStream` with an audio-only configuration (`capturesAudio=YES`). Two real-world macOS behaviors force decisions in the capture adapter (`internal/adapter/audio/capture_darwin_objc.go`):

1. **`SCContentFilter` scope.** The audio-only stream must select a capture source. The textbook choice is `initWithDisplay:display excludingWindows:@[]` ("whole display, exclude nothing"). Community reports (Federico Terzi's writeup, several SDK projects) describe that on some macOS versions this exact form _starts the stream but never delivers buffers_ — silence with no error. The reliable variant is `initWithDisplay:includingApplications:exceptingWindows:`, passing the list of running applications.

2. **Code-signature requirement in dev.** `SCShareableContent` enumerates fine and `CGPreflightScreenCaptureAccess()` returns true (TCC is satisfied via the responsible process, ADR-008), but `startCaptureWithCompletionHandler:` fails on macOS Sequoia/Tahoe for **unsigned or ad-hoc-signed** binaries with `SCError 1003` (`CoreGraphicsErrorDomain`, `kCGErrorInvalidConnection`). Additionally, TCC screen-recording grants are keyed to the app's code signature (ADR-008), so an ad-hoc identity — which changes on every rebuild — silently drops the grant. Microphone capture (AVFoundation) has no such signature validation, which is why it works while system sound fails.

   Empirically verified on macOS 26.6 (Tahoe): **a self-signed identity is also insufficient.** Both TCC grant honoring (`CGPreflightScreenCaptureAccess()` stays false for the bundle) and the SCK daemon (1003 at `startCapture` even when the grant is satisfied via the responsible process) require an **Apple-issued certificate carrying a Team ID** (a free "Apple Development" cert from a personal team is enough). SCK signing requirements also apply to screen/OCR capture (stage 4), so this wall is not audio-specific.

3. **Stream configuration dimensions.** Even with a correctly signed, entitled binary and a satisfied screen-recording grant, `startCapture` fails with `SCError 1003` (`kCGErrorInvalidConnection`) and failure reason **"The stream is nil."** if the `SCStreamConfiguration` uses `width = 1; height = 1` (the textbook "audio-only" setup). The daemon cannot materialize the internal video/session pipeline from a 1×1 config on Tahoe. Set `width`/`height` to the source display's real dimensions and add explicit `sampleRate`/`channelCount` (48 kHz, 2 ch) plus a sane `queueDepth`; the stream then starts and delivers system-audio buffers. This was the final blocker after signing + TCC + grants were all green (verified via `error.userInfo`, which the capture adapter now surfaces: `NSLocalizedFailureReason = "The stream is nil."`).

## Options

### Filter scope

**A. `initWithDisplay:excludingWindows:@[]`**

Pros: textbook, minimal.
Cons: on some macOS versions the stream starts but delivers no audio buffers; display-level filters behave differently on Tahoe.

**B. `initWithDisplay:includingApplications:content.applications exceptingWindows:@[]`**

Pros: reliably delivers buffers (known-good across versions, incl. Tahoe); still captures all system audio on the display.
Cons: enumerates the application list once at stream start (negligible, one call).

### Dev signing

**A. Unsigned / ad-hoc / self-signed dev builds**

Pros: no setup.
Cons: `startCapture` fails with SCError 1003 on Sequoia/Tahoe; TCC grants don't stick across rebuilds; a self-signed identity (no Team ID) is ignored by TCC even when granted.

**B. Apple Development certificate** (free, from a personal team) + hardened runtime + `com.apple.security.device.screen-capture` entitlement

Pros: carries a Team ID, which both TCC grant honoring and the SCK daemon require on Tahoe; grants persist across rebuilds because the signature is stable; matches documented working setups.
Cons: one-time setup (Xcode + Apple ID sign-in); the dev loop must build+sign+open instead of `wails dev`.

## Selection criteria

| Criterion                      | Filter A | Filter B | Signing A | Signing B |
| ------------------------------ | -------- | -------- | --------- | --------- |
| Reliable system-audio delivery | ⚠️       | ✅       | —         | —         |
| Works on Tahoe dev builds      | —        | —        | ❌        | ✅        |
| Grants persist across rebuilds | —        | —        | ❌        | ✅        |
| Setup / process cost           | ✅       | ✅       | ✅        | ⚠️        |

## Decision

- The capture filter uses `initWithDisplay:includingApplications:exceptingWindows:`.
- Dev builds are signed via `scripts/dev-audio.sh` with an **Apple Development certificate** (hardened runtime + `build/darwin/entitlements.plist` with `com.apple.security.device.screen-capture` and `com.apple.security.device.audio-input` — under hardened runtime the mic is hard-denied by TCC without the audio-input entitlement, even when granted in System Settings); the script auto-picks the Apple Development identity and falls back to `$IA_DEV_SIGN_IDENTITY` / the self-signed cert. Release distribution requires a Developer ID certificate + notarization (planned separately, not part of this ADR).

## Trade-offs

- Including all applications means the content filter is broader than strictly necessary, but it is built once at stream start and has no runtime cost.
- A real-display-width stream config is heavier than a 1×1 "audio-only" config on paper, but the stream is still audio-only (no video output is added); the dimensions only exist so the daemon can create the internal session.
- Dev signing does not cover distribution: release builds need Developer ID + notarization (and the same entitlement), which stays a separate pipeline concern.
- `wails dev` (hot reload) cannot be used for audio testing on Tahoe because it launches the unsigned bare binary; audio work goes through `scripts/dev-audio.sh`.
- The free Apple Development certificate chains to the Apple WWDR G3 intermediate; if that intermediate / "Apple Root CA - G3" anchor is missing from the keychain, `security find-identity -v` marks the identity not-valid (codesign still works, but TCC may not honor the grant). Install the WWDR G3 + root from Apple's certificate-authority page if a signed build is still rejected.
