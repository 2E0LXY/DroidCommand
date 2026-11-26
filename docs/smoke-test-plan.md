# Android smoke-test acceptance plan

Draft pending test APK and CLI contracts. Run first against a fake ADB runner, then a dedicated authorised handset or emulator containing only synthetic test data. Do not run against personal apps.

| Case | Expected outcome |
| --- | --- |
| No device, offline, unauthorised or multiple devices | Clear failure; no input sent |
| Serial not allowlisted | Refusal before interaction |
| Locked or unknown lock state | Refusal |
| Unapproved or unknown foreground package | No screenshot, UI dump or input |
| Launch disposable test app | Expected package and stable screen marker |
| Tap a test button | Expected synthetic status marker |
| Enter synthetic text | Exact expected test value |
| Swipe test list | Expected item visible |
| Capture screenshot and UI tree | Local artifact only, no personal data |
| ADB timeout or malformed output | Bounded failure, no automatic replay of mutations |
| Concurrent actions | Serialized per device; no interleaving |
| Session cancellation | Further input refused |

Use a deterministic fixture with stable accessibility identifiers, a button, a synthetic text field and a scrollable list. Record build version, test ID, pass/fail, duration and sanitised error category. Keep device identifiers and screenshots out of public CI logs. CI should use fake ADB by default; physical-device runs require explicit local opt-in. No test results are claimed yet.
