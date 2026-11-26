// Package mcp is a minimal stdio MCP server (JSON-RPC 2.0, newline-delimited).
// It exposes only the guarded dc operations; there is no shell passthrough
// and no network listener.
package mcp

import (
	"bufio"
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"sync"

	"github.com/2E0LXY/DroidCommand/internal/dc"
	"github.com/2E0LXY/DroidCommand/internal/guard"
)

const (
	protocolVersion = "2024-11-05"
	maxLine         = 1 << 20 // 1 MiB request limit
)

// Server serves MCP over a reader/writer pair. Requests are handled one at a
// time, so device operations never run concurrently.
type Server struct {
	C         *dc.Client
	AllowText bool // owner opt-in at launch; the `text` tool is hidden otherwise
	Version   string

	mu  sync.Mutex // guards writes to out
	out io.Writer
}

type request struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id,omitempty"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

type rpcError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

type response struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id"`
	Result  any             `json:"result,omitempty"`
	Error   *rpcError       `json:"error,omitempty"`
}

type content struct {
	Type     string `json:"type"`
	Text     string `json:"text,omitempty"`
	Data     string `json:"data,omitempty"`
	MimeType string `json:"mimeType,omitempty"`
}

type toolResult struct {
	Content []content `json:"content"`
	IsError bool      `json:"isError,omitempty"`
}

type tool struct {
	Name        string         `json:"name"`
	Description string         `json:"description"`
	InputSchema map[string]any `json:"inputSchema"`
}

func props(kv ...any) map[string]any {
	p := map[string]any{}
	for i := 0; i < len(kv); i += 2 {
		p[kv[i].(string)] = map[string]any{"type": kv[i+1]}
	}
	return p
}

func schema(required []string, kv ...any) map[string]any {
	return map[string]any{"type": "object", "properties": props(kv...), "required": required, "additionalProperties": false}
}

func (s *Server) tools() []tool {
	t := []tool{
		{"devices", "List attached, allowlisted devices in state 'device'.", schema([]string{})},
		{"screenshot", "PNG screenshot of an allowlisted device. Refused if locked or the foreground app is not allowlisted.", schema([]string{"serial"}, "serial", "string")},
		{"ui_dump", "uiautomator XML of the current screen (same refusals as screenshot).", schema([]string{"serial"}, "serial", "string")},
		{"tap", "Tap at screen coordinates (0-10000).", schema([]string{"serial", "x", "y"}, "serial", "string", "x", "integer", "y", "integer")},
		{"swipe", "Swipe between coordinates over duration_ms (50-5000).", schema([]string{"serial", "x1", "y1", "x2", "y2", "duration_ms"}, "serial", "string", "x1", "integer", "y1", "integer", "x2", "integer", "y2", "integer", "duration_ms", "integer")},
		{"start_app", "Launch an allowlisted package.", schema([]string{"serial", "package"}, "serial", "string", "package", "string")},
	}
	if s.AllowText {
		t = append(t, tool{"text", "Type text [A-Za-z0-9 .,@_-], max 256 chars. Never for credentials. Enabled by the owner at launch only.", schema([]string{"serial", "text"}, "serial", "string", "text", "string")})
	}
	return t
}

// Serve reads requests until EOF or ctx cancellation.
func (s *Server) Serve(ctx context.Context, in io.Reader, out io.Writer) error {
	s.out = out
	sc := bufio.NewScanner(in)
	sc.Buffer(make([]byte, 0, 64*1024), maxLine)
	for sc.Scan() {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		line := sc.Bytes()
		if len(line) == 0 {
			continue
		}
		s.handle(ctx, line)
	}
	if err := sc.Err(); err != nil {
		if errors.Is(err, bufio.ErrTooLong) {
			s.reply(nil, nil, &rpcError{-32600, "request too large"})
		}
		return err
	}
	return nil
}

func (s *Server) reply(id json.RawMessage, result any, e *rpcError) {
	if id == nil {
		id = json.RawMessage("null")
	}
	b, _ := json.Marshal(response{JSONRPC: "2.0", ID: id, Result: result, Error: e})
	s.mu.Lock()
	defer s.mu.Unlock()
	s.out.Write(append(b, '\n'))
}

func (s *Server) handle(ctx context.Context, line []byte) {
	var r request
	if err := json.Unmarshal(line, &r); err != nil || r.JSONRPC != "2.0" || r.Method == "" {
		s.reply(nil, nil, &rpcError{-32700, "parse error"})
		return
	}
	if r.ID == nil { // notification: never answered
		return
	}
	switch r.Method {
	case "initialize":
		s.reply(r.ID, map[string]any{
			"protocolVersion": protocolVersion,
			"capabilities":    map[string]any{"tools": map[string]any{}},
			"serverInfo":      map[string]any{"name": "droidcommand", "version": s.Version},
		}, nil)
	case "ping":
		s.reply(r.ID, map[string]any{}, nil)
	case "tools/list":
		s.reply(r.ID, map[string]any{"tools": s.tools()}, nil)
	case "tools/call":
		var p struct {
			Name      string          `json:"name"`
			Arguments json.RawMessage `json:"arguments"`
		}
		if err := json.Unmarshal(r.Params, &p); err != nil || p.Name == "" {
			s.reply(r.ID, nil, &rpcError{-32602, "invalid params"})
			return
		}
		s.reply(r.ID, s.call(ctx, p.Name, p.Arguments), nil)
	default:
		s.reply(r.ID, nil, &rpcError{-32601, "method not found"})
	}
}

func errResult(code, msg string) toolResult {
	b, _ := json.Marshal(map[string]string{"error": code, "message": msg})
	return toolResult{Content: []content{{Type: "text", Text: string(b)}}, IsError: true}
}

func okText(v any) toolResult {
	b, _ := json.Marshal(v)
	return toolResult{Content: []content{{Type: "text", Text: string(b)}}}
}

// classify maps internal errors to stable codes without leaking adb arguments.
func classify(err error) toolResult {
	switch {
	case errors.Is(err, guard.ErrSerial):
		return errResult("serial_denied", err.Error())
	case errors.Is(err, guard.ErrPackage):
		return errResult("package_denied", err.Error())
	case errors.Is(err, guard.ErrText):
		return errResult("invalid_text", err.Error())
	case errors.Is(err, dc.ErrLocked):
		return errResult("device_locked", err.Error())
	case errors.Is(err, dc.ErrForeground):
		return errResult("foreground_denied", err.Error())
	case errors.Is(err, dc.ErrUnknownState):
		return errResult("unknown_state", err.Error())
	case errors.Is(err, context.DeadlineExceeded):
		return errResult("timeout", "adb operation timed out")
	case errors.Is(err, context.Canceled):
		return errResult("cancelled", "operation cancelled")
	}
	// Never forward raw adb errors: they embed arguments and device stderr. Detail stays in the audit log.
	return errResult("adb_failed", "adb command failed; see the audit log")
}

func (s *Server) call(ctx context.Context, name string, raw json.RawMessage) toolResult {
	// Resolve the tool before looking at arguments so unknown tools are reported as such.
	switch name {
	case "devices", "screenshot", "ui_dump", "tap", "swipe", "start_app":
	case "text":
		if !s.AllowText {
			return errResult("tool_disabled", "text entry was not enabled by the owner at launch")
		}
	default:
		return errResult("unknown_tool", fmt.Sprintf("no such tool %q", name))
	}
	var a struct {
		Serial     string `json:"serial"`
		Package    string `json:"package"`
		Text       string `json:"text"`
		X          *int   `json:"x"`
		Y          *int   `json:"y"`
		X1         *int   `json:"x1"`
		Y1         *int   `json:"y1"`
		X2         *int   `json:"x2"`
		Y2         *int   `json:"y2"`
		DurationMS *int   `json:"duration_ms"`
	}
	if len(raw) > 0 {
		// Strict: a JSON object only (not null, array or scalar), known fields only, documented types only.
		if t := bytes.TrimSpace(raw); len(t) == 0 || t[0] != '{' {
			return errResult("invalid_input", "arguments must be a JSON object")
		}
		dec := json.NewDecoder(bytes.NewReader(raw))
		dec.DisallowUnknownFields()
		if err := dec.Decode(&a); err != nil {
			return errResult("invalid_input", "arguments contain unknown fields or wrong types")
		}
	}
	need := func(v ...*int) bool {
		for _, p := range v {
			if p == nil {
				return false
			}
		}
		return true
	}
	bad := func(m string) toolResult { return errResult("invalid_input", m) }
	switch name {
	case "devices":
		d, err := s.C.Devices(ctx)
		if err != nil {
			return classify(err)
		}
		if d == nil {
			d = []string{}
		}
		return okText(map[string]any{"devices": d})
	case "screenshot":
		b, err := s.C.Screenshot(ctx, a.Serial)
		if err != nil {
			return classify(err)
		}
		return toolResult{Content: []content{{Type: "image", Data: base64.StdEncoding.EncodeToString(b), MimeType: "image/png"}}}
	case "ui_dump":
		b, err := s.C.UIDump(ctx, a.Serial)
		if err != nil {
			return classify(err)
		}
		return toolResult{Content: []content{{Type: "text", Text: string(b)}}}
	case "tap":
		if !need(a.X, a.Y) {
			return bad("x and y are required integers")
		}
		if err := s.C.Tap(ctx, a.Serial, *a.X, *a.Y); err != nil {
			return classify(err)
		}
	case "swipe":
		if !need(a.X1, a.Y1, a.X2, a.Y2, a.DurationMS) {
			return bad("x1, y1, x2, y2 and duration_ms are required integers")
		}
		if err := s.C.Swipe(ctx, a.Serial, *a.X1, *a.Y1, *a.X2, *a.Y2, *a.DurationMS); err != nil {
			return classify(err)
		}
	case "start_app":
		if err := s.C.StartApp(ctx, a.Serial, a.Package); err != nil {
			return classify(err)
		}
	case "text":
		if !s.AllowText {
			return errResult("tool_disabled", "text entry was not enabled by the owner at launch")
		}
		// Approval is the owner's launch flag, not a model-supplied argument.
		if err := s.C.Text(ctx, a.Serial, a.Text, true); err != nil {
			return classify(err)
		}
	default:
		return errResult("unknown_tool", fmt.Sprintf("no such tool %q", name))
	}
	return okText(map[string]bool{"ok": true})
}
