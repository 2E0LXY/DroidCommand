# Windows Android test-host onboarding

Status: proposed; hardware validation pending. DroidCommand uses Go and Android SDK Platform-Tools, not Python or Phone Harness at runtime.

## Host and device prerequisites

Use a dedicated Android test handset or emulator with no personal accounts or sensitive data. Obtain the owner's permission before connecting a device. Install current Android SDK Platform-Tools from https://developer.android.com/tools/releases/platform-tools and add its extracted directory to the current user's PATH. Verify in PowerShell with `adb version`. For source builds, install the Go version specified by the project's eventual go.mod. Android APK builds use the project's Gradle wrapper and CI.

## Connection and verification

Prefer USB and a data-capable cable. The device owner enables Developer options and USB debugging, connects the cable and personally approves the host fingerprint. Run `adb devices -l` locally; only proceed with the explicitly authorised test device in `device` state. Stop for unauthorized, offline, ambiguous or missing devices. Do not attempt to unlock a device programmatically. Windows may require the manufacturer's USB driver.

Wireless debugging is optional on supported Android versions: use Android's built-in pairing flow on a trusted private network with the owner present. Never forward ADB ports or expose them to the internet. Do not enable legacy unauthenticated TCP debugging. Disable wireless debugging when finished.

## DroidCommand configuration

The final configuration syntax will be documented after the Go implementation is reviewed. Require an explicit device serial and the disposable test application's package identifier; do not auto-select a handset. Keep configuration and artifacts outside the public repository. MCP transport is local stdio only. No raw shell tool, no cloud accounts, no telemetry.

## Acceptance and teardown

Check device discovery and lock state; run read-only inspection and then a harmless interaction inside the test app. Stop on unknown foreground package or lock state. Do not capture personal apps, notifications or credential fields. Store test artifacts locally with limited retention; never commit screenshots or device identifiers. At session end disconnect the cable, revoke debugging authorisation if no longer required, and disable debugging. No device has been connected or tested as part of writing this document.
