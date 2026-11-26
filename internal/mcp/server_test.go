package mcp

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"github.com/2E0LXY/DroidCommand/internal/dc"
	"github.com/2E0LXY/DroidCommand/internal/guard"
)

const pkg = "uk.co.droidcommand.testapp"

type fakeADB struct {
	locked bool
	inputs int
	err    error
}

func (f *fakeADB) Run(_ context.Context, args ...string) ([]byte, error) {
	j := strings.Join(args, " ")
	switch {
	case j == "devices":
		return []byte("List of devices attached\nABC123\tdevice\nEVIL9\tdevice\n"), nil
	case strings.Contains(j, "dumpsys window"):
		return []byte(fmt.Sprintf("mDreamingLockscreen=%v\nmCurrentFocus=Window{1 u0 %s/%s.Main}\n", f.locked, pkg, pkg)), nil
	case strings.Contains(j, "screencap"):
		return []byte("\x89PNGdata"), nil
	}
	if strings.Contains(j, " input ") {
		f.inputs++
		if f.err != nil {
			return nil, f.err
		}
	}
	return []byte("ok"), nil
}

func run(t *testing.T, f *fakeADB, allowText bool, reqs ...string) []map[string]any {
	t.Helper()
	s := &Server{C: &dc.Client{R: f, Cfg: guard.Config{Serials: []string{"ABC123"}, Packages: []string{pkg}}}, AllowText: allowText, Version: "test"}
	var out bytes.Buffer
	if err := s.Serve(context.Background(), strings.NewReader(strings.Join(reqs, "\n")+"\n"), &out); err != nil {
		t.Fatal(err)
	}
	var res []map[string]any
	for _, ln := range strings.Split(strings.TrimSpace(out.String()), "\n") {
		if ln == "" {
			continue
		}
		var m map[string]any
		if err := json.Unmarshal([]byte(ln), &m); err != nil {
			t.Fatalf("bad json %q: %v", ln, err)
		}
		res = append(res, m)
	}
	return res
}

func call(id int, name, args string) string {
	return fmt.Sprintf(`{"jsonrpc":"2.0","id":%d,"method":"tools/call","params":{"name":%q,"arguments":%s}}`, id, name, args)
}

func result(m map[string]any) map[string]any { return m["result"].(map[string]any) }

func firstText(m map[string]any) string {
	return result(m)["content"].([]any)[0].(map[string]any)["text"].(string)
}

func errCode(t *testing.T, m map[string]any) string {
	t.Helper()
	if result(m)["isError"] != true {
		t.Fatalf("expected tool error, got %v", m)
	}
	var e map[string]string
	json.Unmarshal([]byte(firstText(m)), &e)
	return e["error"]
}

func TestInitializeAndList(t *testing.T) {
	r := run(t, &fakeADB{}, false,
		`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{}}`,
		`{"jsonrpc":"2.0","method":"notifications/initialized"}`,
		`{"jsonrpc":"2.0","id":2,"method":"tools/list"}`,
		`{"jsonrpc":"2.0","id":3,"method":"ping"}`)
	if len(r) != 3 { // the notification gets no reply
		t.Fatalf("got %d replies", len(r))
	}
	if result(r[0])["protocolVersion"] != protocolVersion {
		t.Fatal("protocol version")
	}
	for _, tl := range result(r[1])["tools"].([]any) {
		if tl.(map[string]any)["name"] == "text" {
			t.Fatal("text tool must be hidden without -allow-text")
		}
	}
	r = run(t, &fakeADB{}, true, `{"jsonrpc":"2.0","id":1,"method":"tools/list"}`)
	found := false
	for _, tl := range result(r[0])["tools"].([]any) {
		found = found || tl.(map[string]any)["name"] == "text"
	}
	if !found {
		t.Fatal("text tool expected with allow-text")
	}
}

func TestToolsHappyPath(t *testing.T) {
	r := run(t, &fakeADB{}, false,
		call(1, "devices", `{}`),
		call(2, "tap", `{"serial":"ABC123","x":10,"y":20}`),
		call(3, "screenshot", `{"serial":"ABC123"}`),
		call(4, "start_app", fmt.Sprintf(`{"serial":"ABC123","package":%q}`, pkg)))
	if !strings.Contains(firstText(r[0]), `"ABC123"`) || strings.Contains(firstText(r[0]), "EVIL9") {
		t.Fatalf("devices: %s", firstText(r[0]))
	}
	if result(r[1])["isError"] == true {
		t.Fatal("tap failed")
	}
	img := result(r[2])["content"].([]any)[0].(map[string]any)
	if img["type"] != "image" || img["mimeType"] != "image/png" {
		t.Fatalf("screenshot content %v", img)
	}
}

func TestSecurityRefusalsAreStructured(t *testing.T) {
	f := &fakeADB{}
	r := run(t, f, false,
		call(1, "tap", `{"serial":"EVIL9","x":1,"y":1}`),
		call(2, "start_app", `{"serial":"ABC123","package":"com.bank.app"}`),
		call(3, "text", `{"serial":"ABC123","text":"hi"}`),
		call(4, "shell", `{"cmd":"id"}`))
	want := []string{"serial_denied", "package_denied", "tool_disabled", "unknown_tool"}
	for i, w := range want {
		if got := errCode(t, r[i]); got != w {
			t.Errorf("req %d: got %s want %s", i+1, got, w)
		}
	}
	if f.inputs != 0 {
		t.Fatal("no input may reach adb")
	}
}

func TestLockedDeviceRefused(t *testing.T) {
	f := &fakeADB{locked: true}
	r := run(t, f, false, call(1, "tap", `{"serial":"ABC123","x":1,"y":1}`))
	if errCode(t, r[0]) != "device_locked" || f.inputs != 0 {
		t.Fatal("locked device must be refused")
	}
}

func TestInvalidInput(t *testing.T) {
	r := run(t, &fakeADB{}, false,
		call(1, "tap", `{"serial":"ABC123","x":1}`),
		call(2, "tap", `{"serial":"ABC123","x":"a","y":1}`),
		call(3, "swipe", `{"serial":"ABC123","x1":1,"y1":1,"x2":2,"y2":2}`),
		`{"jsonrpc":"2.0","id":4,"method":"tools/call","params":{}}`)
	if errCode(t, r[0]) != "invalid_input" || errCode(t, r[1]) != "invalid_input" || errCode(t, r[2]) != "invalid_input" {
		t.Fatal("expected invalid_input")
	}
	if r[3]["error"].(map[string]any)["code"].(float64) != -32602 {
		t.Fatal("expected -32602")
	}
}

func TestProtocolErrors(t *testing.T) {
	r := run(t, &fakeADB{}, false, `not json`, `{"jsonrpc":"2.0","id":1,"method":"nope"}`)
	if r[0]["error"].(map[string]any)["code"].(float64) != -32700 {
		t.Fatal("parse error expected")
	}
	if r[1]["error"].(map[string]any)["code"].(float64) != -32601 {
		t.Fatal("method not found expected")
	}
}

func TestTextEnabledAndPayloadNotLeaked(t *testing.T) {
	f := &fakeADB{err: fmt.Errorf("boom")}
	r := run(t, f, true, call(1, "text", `{"serial":"ABC123","text":"hello world"}`))
	if code := errCode(t, r[0]); code != "adb_failed" {
		t.Fatalf("code %s", code)
	}
	if strings.Contains(firstText(r[0]), "hello") {
		t.Fatal("payload leaked in error")
	}
	r = run(t, &fakeADB{}, true, call(1, "text", `{"serial":"ABC123","text":"a;reboot"}`))
	if errCode(t, r[0]) != "invalid_text" {
		t.Fatal("charset must be enforced")
	}
}

func TestCancelledContext(t *testing.T) {
	s := &Server{C: &dc.Client{R: &fakeADB{}, Cfg: guard.Config{Serials: []string{"ABC123"}, Packages: []string{pkg}}}}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	var out bytes.Buffer
	if err := s.Serve(ctx, strings.NewReader(call(1, "devices", `{}`)+"\n"), &out); err == nil {
		t.Fatal("expected context error")
	}
	if out.Len() != 0 {
		t.Fatal("no work after cancellation")
	}
}

func TestOversizedRequestRejected(t *testing.T) {
	s := &Server{C: &dc.Client{R: &fakeADB{}}}
	var out bytes.Buffer
	big := strings.Repeat("a", maxLine+10)
	if err := s.Serve(context.Background(), strings.NewReader(big+"\n"), &out); err == nil {
		t.Fatal("expected error for oversized line")
	}
	if !strings.Contains(out.String(), "request too large") {
		t.Fatalf("out %q", out.String())
	}
}

func TestSequentialRequestsKeepOrder(t *testing.T) {
	var reqs []string
	for i := 1; i <= 20; i++ {
		reqs = append(reqs, call(i, "devices", `{}`))
	}
	r := run(t, &fakeADB{}, false, reqs...)
	for i, m := range r {
		if int(m["id"].(float64)) != i+1 {
			t.Fatalf("order broken at %d", i)
		}
	}
}

func TestStrictArguments(t *testing.T) {
	f := &fakeADB{}
	r := run(t, f, false,
		call(1, "tap", `{"serial":"ABC123","x":1,"y":1,"extra":true}`),
		call(2, "tap", `null`),
		call(3, "tap", `[1,2]`),
		call(4, "tap", `"x"`),
		call(5, "tap", `{"serial":"ABC123","x":1.5,"y":1}`),
		call(6, "devices", `{"serial":"ABC123","bogus":1}`))
	for i, m := range r {
		if errCode(t, m) != "invalid_input" {
			t.Errorf("req %d: want invalid_input, got %s", i+1, firstText(m))
		}
	}
	if f.inputs != 0 {
		t.Fatal("no input may reach adb for malformed arguments")
	}
}

func TestAdbErrorsAreSanitised(t *testing.T) {
	f := &fakeADB{err: fmt.Errorf("adb [-s ABC123 shell input tap 1 2]: exit status 1: device stderr secret")}
	r := run(t, f, false, call(1, "tap", `{"serial":"ABC123","x":1,"y":2}`))
	if errCode(t, r[0]) != "adb_failed" {
		t.Fatal("expected adb_failed")
	}
	if txt := firstText(r[0]); strings.Contains(txt, "secret") || strings.Contains(txt, "ABC123") || strings.Contains(txt, "shell") {
		t.Fatalf("raw adb error leaked: %s", txt)
	}
}
