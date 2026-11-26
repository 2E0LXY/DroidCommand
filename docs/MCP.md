# MCP setup

DroidCommand speaks MCP over stdio only (`droidcommand serve`). There is no HTTP listener and no raw shell tool.

## Claude Code
```
claude mcp add droidcommand -- droidcommand -config /path/to/droidcommand.json serve
```

## Generic client configuration
```json
{
  "mcpServers": {
    "droidcommand": {
      "command": "droidcommand",
      "args": ["-config", "/path/to/droidcommand.json", "serve"]
    }
  }
}
```
On Windows use the full path to `droidcommand.exe` and escape backslashes.

## Tools
| Tool | Arguments | Notes |
| --- | --- | --- |
| `devices` | none | Allowlisted devices in state `device` |
| `screenshot` | `serial` | PNG image content, not written to disk |
| `ui_dump` | `serial` | uiautomator XML |
| `tap` | `serial`, `x`, `y` | 0-10000 |
| `swipe` | `serial`, `x1`, `y1`, `x2`, `y2`, `duration_ms` | 50-5000 ms |
| `start_app` | `serial`, `package` | Allowlisted package only |
| `text` | `serial`, `text` | Only with `serve -allow-text`; `[A-Za-z0-9 .,@_-]`, max 256 |

All tools except `devices` and `start_app` require an unlocked device with an allowlisted app in the foreground. `start_app` still refuses on a locked device.

## Errors
Tool failures return `isError: true` with JSON `{"error": code, "message": ...}`. Codes: `serial_denied`, `package_denied`, `device_locked`, `foreground_denied`, `unknown_state`, `invalid_input`, `invalid_text`, `tool_disabled`, `unknown_tool`, `timeout`, `cancelled`, `adb_failed`.

## Argument handling
`arguments` must be a JSON object containing only documented fields with documented types. Unknown fields, `null`, arrays, scalars and fractional numbers are rejected with `invalid_input`. Unknown tools return `unknown_tool` regardless of arguments.

## Security notes
- The foreground check is not isolation: the screen can change between the check and an input action. Use a dedicated test device only (see INSTALL.md, Residual risk).
- `adb_failed` messages are generic; details are in the audit log, not in tool responses.
- Approval for typing is the owner's `-allow-text` launch flag, never a model argument.
- Do not use `text` for credentials. Do not allowlist personal or banking apps.
- Requests run one at a time; each adb call has a 30 s timeout.
