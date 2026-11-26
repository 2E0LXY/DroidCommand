# Installation

DroidCommand drives one authorised Android test device over local adb. Nothing listens on the network and nothing is sent to any cloud service.

## Prerequisites
- Windows 10/11 (x64) or Linux (x64 or arm64).
- Android platform-tools (`adb`) on `PATH`. Check with `droidcommand doctor`.
- A dedicated Android test device (Android 8.0+), with Developer options and USB debugging enabled. Authorise the computer on the device when prompted.
- Do not use a personal phone. Never expose adb over the network (`adb tcpip`).

## Install
1. Download the archive for your platform from the GitHub release, plus `SHA256SUMS`.
2. Verify: `sha256sum -c SHA256SUMS --ignore-missing` (Linux) or `Get-FileHash droidcommand-windows-amd64.exe -Algorithm SHA256` (Windows) and compare.
3. Put the binary somewhere on `PATH`.

## Install the test app
1. Download `droidcommand-testapp-debug.apk` from the same release and verify its checksum.
2. `adb -s <serial> install droidcommand-testapp-debug.apk` (the owner does this; DroidCommand never installs, removes or clears apps).

## Configure
Copy `droidcommand.example.json` to `droidcommand.json`:

```json
{
  "serials": ["<output of adb devices>"],
  "packages": ["uk.co.droidcommand.testapp"],
  "audit_log": "audit.jsonl"
}
```

Empty `serials` or `packages` deny everything. Add only packages you own and are authorised to test.

## Try it
```
droidcommand devices
droidcommand -serial <serial> start-app uk.co.droidcommand.testapp
droidcommand -serial <serial> screenshot shot.png
```
The device must be unlocked by you. DroidCommand refuses on a locked screen, when lock state cannot be read, or when the foreground app is not allowlisted. Screenshots and UI dumps may contain sensitive data: keep them local.

## Verification status
Unit tests use a fake adb. Nothing has been verified on physical hardware yet.
