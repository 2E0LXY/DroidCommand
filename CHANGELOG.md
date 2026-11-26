# Changelog

## [Unreleased]

## [0.1.0]

First development preview. Not tested on physical hardware.

### Added
- Guarded local adb wrapper in Go: device serial and package allowlists (empty denies all), lock-screen refusal, foreground-app check, JSONL audit log. No shell passthrough.
- CLI commands: `version`, `doctor`, `devices`, `screenshot`, `ui`, `tap`, `swipe`, `text`, `start-app`, `serve`.
- Local stdio MCP server (`serve`) with `devices`, `screenshot`, `ui_dump`, `tap`, `swipe`, `start_app`. `text` is available only when started with `-allow-text`.
- Disposable Kotlin/Compose test app `uk.co.droidcommand.testapp` (no permissions, no network, stable test tags). The release APK is debug-signed.
- GitHub Actions: Go checks, Android build, release workflow with SHA-256 checksums.

### Security
- Lock and foreground state fail closed: missing, non-boolean or conflicting markers are refused. Screenshots and UI dumps are re-checked after capture and discarded if state changed.
- MCP arguments are strict (unknown fields, null and non-objects rejected); adb errors are sanitised in tool responses. Typed text never reaches the audit log or error messages.

### Known limitations
- The foreground check is a guard, not isolation; the screen can change between check and action. Dedicated test devices only.
- Device-state detection (`dumpsys window`) is unverified on real devices; failure mode is refusal.
- Windows and Linux binaries only; no macOS build.
