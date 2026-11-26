# Android smoke-test acceptance plan

v0.1.0 acceptance design; physical execution pending. Run first against a fake ADB runner, then a dedicated authorised handset or emulator containing only synthetic test data. Do not run against personal apps.

| Case | Expected outcome |
| --- | --- |
| No device, offline or unauthorised device | Not returned as available; no input sent |
| Serial not allowlisted | Refusal before interaction |
| Locked or unknown lock state | Refusal |
| Unapproved or unknown foreground package | No screenshot, UI dump or input |
| Launch disposable test app | Expected package and stable screen marker |
| Tap a test button | Expected synthetic status marker |
| Enter synthetic text | Exact expected test value |
| Swipe within fixture | Safe bounded operation; scrolling assertion deferred until a scrollable fixture exists |
| Capture screenshot and UI tree | Local artifact only, no personal data |
| ADB timeout or malformed output | Bounded failure, no automatic replay of mutations |
| Concurrent actions | Serialized per device; no interleaving |
| Session cancellation | Further input refused |

Use a deterministic fixture with stable accessibility identifiers, a button, a synthetic text field and two navigation screens. Record build version, test ID, pass/fail, duration and sanitised error category. Keep device identifiers and screenshots out of public CI logs. CI should use fake ADB by default; physical-device runs require explicit local opt-in. Go and APK CI passed; physical acceptance results remain unverified. For each case record expected and observed result, pass/fail/not run, and stop on unexpected device state. Use the stable fixture tags title, counter, increment, input, echo, nav_next, nav_back and screen_two. Foreground checks are guards, not isolation; use only disposable test devices. Do not retain or transmit sensitive content.
