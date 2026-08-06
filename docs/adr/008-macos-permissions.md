# ADR-008: macOS permissions — microphone, screen recording, Accessibility

**Date:** 2026-08-01
**Status:** Accepted
**Related documents:** 01-tz.md §7/§8, 02-nfr.md (NFR-05, NFR-06), 05-architecture.md §4, ADR-006, ADR-007

---

## Context

For v1 (macOS 14+) the app needs system permissions:

| Permission                                                | Why                                                 | When                   |
| --------------------------------------------------------- | --------------------------------------------------- | ---------------------- |
| Microphone (`NSMicrophoneUsageDescription`)               | Microphone capture (role `user`, ADR-007)           | Stage 3                |
| Screen recording (`CGPreflightScreenCaptureAccess` / SCK) | System sound capture via ScreenCaptureKit (ADR-007) | Stage 3 (system sound) |
| Accessibility (`AXIsProcessTrusted`)                      | Global shortcuts (CGEventTap, ADR-006), NFR-05      | Stage 2                |

The problem: macOS permissions are granted once in System Settings; a second request after denial is impossible programmatically. The app must:

1. Check the status at startup and before use.
2. Clearly explain to the user why the permission is needed and how to grant it (open the relevant Settings pane).
3. Not crash on denial — show the status and degrade (NFR-06): without the microphone the audio scenario is unavailable, without screen recording — system sound, without Accessibility — global shortcuts (in-window ones remain).

## Options

### A. Centralized permissions module + `app:permission` event [chosen]

- `internal/port/permissions.go`:

```
type Permission string
const (
    PermissionMicrophone    Permission = "microphone"
    PermissionScreenCapture Permission = "screen-recording"
    PermissionAccessibility Permission = "accessibility"
)

interface Permissions {
    Status(p Permission) (bool, error)          // granted or not
    Request(p Permission) error                 // request (system dialog call)
    OpenSettings(p Permission) error            // open the relevant System Settings pane
}
```

- `usecase/permissions`: at startup collects statuses → `app:permission { permission, granted }` event.
- Adapter `adapter/system/` (macOS platform calls).
- Frontend: onboarding panel in settings; when a permission is missing — a clear card with an "Open Settings" button. On returning to the app the status is re-checked.
- Info.plist: `NSMicrophoneUsageDescription` (explanation text). There is **no** usage-description key for screen recording — TCC grants for ScreenCaptureKit are keyed to the app's code-signature identity and are granted via System Settings (Screen & System Audio Recording pane), not via Info.plist.

**Pros:**

- A single point for check/status, simple degradation logic.
- The UI lives on the frontend, status arrives via an event (single async channel, 04-events).

**Cons:**

- Some code at the start; some checks are duplicated in adapters (audio/screenshot).

### B. Check permissions in each adapter in place

**Pros:**

- Fewer abstractions.

**Cons:**

- Status is scattered, UI onboarding and tests are harder (NFR-06 — every adapter needs handling). Rejected.

## Selection criteria

| Criterion                 | A (centralized) | B (in place) |
| ------------------------- | --------------- | ------------ |
| Clear UX onboarding       | ✅              | ⚠️           |
| Degradation without crash | ✅              | ⚠️           |
| Testability (NFR-06)      | ✅              | ⚠️           |
| Code volume               | ⚠️              | ✅           |

## Decision

Option **A** chosen — the `Permissions` module + `app:permission` event.

### Default behavior

- At startup: check the status of all three permissions → `app:permission` for each.
- On denial: the app works, but the corresponding feature is unavailable, with an explanation and an "Open Settings" button in settings.
- On returning to the app (window focus) — statuses are re-checked.
- No repeated system request after denial (forbidden by macOS) — only navigation to System Settings.

### Rule for NFR-06

The absence of any permission does not crash the app: the audio scenario requires the microphone; system sound requires screen recording; global shortcuts require Accessibility (otherwise — only in-window/interactive mode).

## Trade-offs

- Screen recording on macOS 14+ also provides system audio — for v1 we use one access (ScreenCaptureKit) for both image capture and system sound (stages 3–4).
- Accessibility for hotkeys is the most "scary" permission for the user; an explanation text is mandatory (why it is needed, that the app only reads shortcuts).
