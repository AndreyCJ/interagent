# ADR-006: Overlay and Hotkeys ports

**Date:** 2026-08-01
**Status:** Accepted
**Related documents:** 01-tz.md §7/§8 (stage 2), 02-nfr.md (NFR-04, NFR-05), 05-architecture.md §2/§4, 06-bind-contracts.md

---

## Context

The overlay window and global shortcuts are the core of the product (01-tz §1, stage 2). Requirements:

- Click-through by default, **toggleable interactive mode** via a global shortcut (owner's decision, 2026-08-01). In interactive mode, manual input and settings are available in the same window.
- Global shortcuts work when the window is not focused, including in fullscreen (NFR-05).

Earlier `05-architecture.md §4` said the window "is not modeled as a port" (managed via Wails + platform calls). This blocks TDD: stage 2 requires "tests on overlay behavior (show/hide, click-through, shortcuts)" (01-tz:115), and testing a usecase without a port is impossible.

## Options

### A. Port Overlay and Hotkeys via ports [chosen]

Add to `internal/port/`:

```
// overlay.go
type OverlayMode string
const (
    OverlayModeClickThrough  OverlayMode = "click-through"
    OverlayModeInteractive   OverlayMode = "interactive"
)

interface Overlay {
    Show() error
    Hide() error
    Toggle() error
    SetMode(mode OverlayMode) error
    GetMode() (OverlayMode, error)
}

// hotkeys.go
interface Hotkeys {
    Register(id string, keys []string) error
    Unregister(id string) error
}
```

- `usecase/overlay` — business logic: mode toggling, validation, `overlay:mode` event generation.
- `usecase/hotkeys` — registration of shortcuts from `AppSettings.Shortcuts`, `id` → action mapping.
- Adapters: `adapter/window/` (Wails + platform calls: `ignoresMouseEvents`, level, hotkey registration) and `adapter/hotkeys/` (CGEventTap / Carbon RegisterEventHotKey).
- Bind: `OverlayBind`, `HotkeysBind` (methods in `06-bind-contracts.md`).

**Pros:**

- The usecase is tested with mocks (stage-2 TDD).
- Clickability mode is an explicit entity, not scattered native calls.

**Cons:**

- More code early on; the `window` adapter partially duplicates Wails window management.

### B. Keep the window outside ports (tests only e2e)

**Pros:**

- Fewer abstractions at the start.

**Cons:**

- Stage 2 requires unit tests for overlay behavior; without a port there is nowhere to write them.
- Window modes are not tested in CI (e2e postponed, review item 7).
- NFR-04 (click-through) stays without an automated check in the Go layer.

## Selection criteria

| Criterion                  | A (ports) | B (outside ports) |
| -------------------------- | --------- | ----------------- |
| Stage-2 TDD tests          | ✅        | ❌                |
| Click-through testability  | ✅        | ⚠️ (e2e only)     |
| Explicit window mode model | ✅        | ❌                |
| Code volume at the start   | ⚠️        | ✅                |

## Decision

Option **A** chosen — introduce the `Overlay` and `Hotkeys` ports and the corresponding usecases. Stage 2 starts with them (tests → implementation).

### Events

- `overlay:mode` — mode change (click-through ⇄ interactive). Data: `{ mode: string }`.
- Interactive mode toggle — global shortcut (default `Cmd+Shift+Space`), registered via `Hotkeys` and configurable in `AppSettings.Shortcuts`.

### Note to NFR-04

NFR-04 is extended: click-through by default, interactive mode via a global shortcut. In interactive mode the window can intercept mouse/keyboard (manual input, settings).

## Trade-offs

- The `window` adapter will contain macOS platform calls (CGEventTap for hotkeys requires Accessibility permission — see ADR-008). They are not covered by unit tests in the Go layer, only e2e/manual checks.
- One global shortcut is occupied by the mode toggle — it cannot be assigned to another action.

## Update (2026-08-03, stage 2)

The native `internal/adapter/window/` adapter was introduced in stage 2 (previously deferred):

- **Transparency** of the window via Wails options (cross-platform): `Mac.WebviewIsTransparent`/`WindowIsTranslucent`, `Windows.WebviewIsTransparent`/`WindowIsTranslucent`, `Linux.WindowIsTranslucent`, `BackgroundColour` with `A=0`; transparent background in CSS.
- **Click-through (macOS):** `apply_darwin.go` (cgo, Obj-C) calls `setIgnoresMouseEvents:` on the main `NSWindow` according to the mode. Windows/Linux — stubs (WS_EX_TRANSPARENT / X11-passthrough deferred).
- **Always-on-top:** `runtime.WindowSetAlwaysOnTop(ctx, true)` in `startup`.
- **Mode toggle:** while the hotkey adapter is a stub, the menu item "Overlay → Toggle Click-through" is used. The global shortcut (ADR-008) remains for the future.
