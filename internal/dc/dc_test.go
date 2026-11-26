package dc

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/2E0LXY/DroidCommand/internal/guard"
)

const testPkg = "uk.co.droidcommand.testapp"

type fake struct {
	calls   [][]string
	locked  bool
	focus   string // package in focus; "" = none reported
	noState bool   // dumpsys output lacks lock markers
	failIn  bool   // `input` commands fail with an error embedding their args
	// switchTo, if set, becomes the focused package once a capture command has run.
	switchTo string
}

func (f *fake) Run(_ context.Context, args ...string) ([]byte, error) {
	f.calls = append(f.calls, args)
	j := strings.Join(args, " ")
	switch {
	case j == "devices":
		return []byte("List of devices attached\nABC123\tdevice\nEVIL9\tdevice\nOFF1\toffline\n"), nil
	case strings.Contains(j, "dumpsys window"):
		if f.noState {
			return []byte("garbage"), nil
		}
		out := fmt.Sprintf("mDreamingLockscreen=%v\n", f.locked)
		if f.focus != "" {
			out += fmt.Sprintf("mCurrentFocus=Window{1a2b u0 %s/%s.MainActivity}\n", f.focus, f.focus)
		}
		return []byte(out), nil
	case f.switchTo != "" && (strings.Contains(j, "screencap") || strings.Contains(j, "uiautomator")):
		f.focus = f.switchTo
		return []byte("captured"), nil
	case f.failIn && strings.Contains(j, " input "):
		return nil, fmt.Errorf("adb %v: exit status 1", args)
	}
	return []byte("ok"), nil
}

func (f *fake) sentInput() bool {
	for _, c := range f.calls {
		if strings.Contains(strings.Join(c, " "), " input ") {
			return true
		}
	}
	return false
}

func newClient(f *fake) *Client {
	if f.focus == "" && !f.noState {
		f.focus = testPkg
	}
	return &Client{R: f, Cfg: guard.Config{Serials: []string{"ABC123"}, Packages: []string{testPkg}}}
}

var ctx = context.Background()

func TestDevicesFiltersAllowlistAndState(t *testing.T) {
	got, _ := newClient(&fake{}).Devices(ctx)
	if len(got) != 1 || got[0] != "ABC123" {
		t.Fatalf("got %v", got)
	}
}

func TestSerialDenied(t *testing.T) {
	f := &fake{}
	if err := newClient(f).Tap(ctx, "EVIL9", 1, 1); !errors.Is(err, guard.ErrSerial) {
		t.Fatalf("err %v", err)
	}
	if len(f.calls) != 0 {
		t.Fatal("adb must not be called for denied serial")
	}
}

func TestLockedRefused(t *testing.T) {
	f := &fake{locked: true}
	if err := newClient(f).Tap(ctx, "ABC123", 1, 1); !errors.Is(err, ErrLocked) {
		t.Fatalf("err %v", err)
	}
	if f.sentInput() {
		t.Fatal("input sent to locked device")
	}
}

func TestUnknownStateFailsClosed(t *testing.T) {
	f := &fake{noState: true}
	c := newClient(f)
	if err := c.Tap(ctx, "ABC123", 1, 1); !errors.Is(err, ErrUnknownState) {
		t.Fatalf("err %v", err)
	}
	if _, err := c.Screenshot(ctx, "ABC123"); !errors.Is(err, ErrUnknownState) {
		t.Fatalf("err %v", err)
	}
	if f.sentInput() {
		t.Fatal("input sent with unknown state")
	}
}

func TestForegroundMustBeAllowlisted(t *testing.T) {
	f := &fake{focus: "com.personal.messenger"}
	c := newClient(f)
	if err := c.Tap(ctx, "ABC123", 1, 1); !errors.Is(err, ErrForeground) {
		t.Fatalf("tap err %v", err)
	}
	if err := c.Swipe(ctx, "ABC123", 1, 1, 2, 2, 100); !errors.Is(err, ErrForeground) {
		t.Fatalf("swipe err %v", err)
	}
	if _, err := c.Screenshot(ctx, "ABC123"); !errors.Is(err, ErrForeground) {
		t.Fatalf("screenshot err %v", err)
	}
	if _, err := c.UIDump(ctx, "ABC123"); !errors.Is(err, ErrForeground) {
		t.Fatalf("ui err %v", err)
	}
	if err := c.Text(ctx, "ABC123", "hi", true); !errors.Is(err, ErrForeground) {
		t.Fatalf("text err %v", err)
	}
	if f.sentInput() {
		t.Fatal("input sent to non-allowlisted foreground app")
	}
}

func TestNoFocusReportedFailsClosed(t *testing.T) {
	f := &fake{}
	c := newClient(f)
	f.focus = "" // dumpsys now reports no focused window
	if err := c.Tap(ctx, "ABC123", 1, 1); !errors.Is(err, ErrUnknownState) {
		t.Fatalf("err %v", err)
	}
}

func TestTextNeedsApprovalAndCharset(t *testing.T) {
	c := newClient(&fake{})
	if err := c.Text(ctx, "ABC123", "hi", false); !errors.Is(err, guard.ErrNeedOK) {
		t.Fatalf("err %v", err)
	}
	if err := c.Text(ctx, "ABC123", "a;reboot", true); !errors.Is(err, guard.ErrText) {
		t.Fatalf("err %v", err)
	}
	if err := c.Text(ctx, "ABC123", "hello world", true); err != nil {
		t.Fatal(err)
	}
}

func TestTextPayloadNeverAudited(t *testing.T) {
	path := t.TempDir() + "/audit.jsonl"
	f := &fake{failIn: true}
	c := newClient(f)
	c.Audit = guard.NewAudit(path)
	err := c.Text(ctx, "ABC123", "secretvalue", true)
	if err == nil || strings.Contains(err.Error(), "secretvalue") {
		t.Fatalf("error must exist and not leak payload: %v", err)
	}
	b, _ := os.ReadFile(path)
	if !strings.Contains(string(b), `"ok":"false"`) || strings.Contains(string(b), "secretvalue") {
		t.Fatalf("audit leaks or is missing: %s", b)
	}
}

func TestStartAppAllowlistAndLock(t *testing.T) {
	c := newClient(&fake{})
	if err := c.StartApp(ctx, "ABC123", "com.bank.app"); !errors.Is(err, guard.ErrPackage) {
		t.Fatalf("err %v", err)
	}
	if err := c.StartApp(ctx, "ABC123", testPkg); err != nil {
		t.Fatal(err)
	}
	// StartApp does not need the target in the foreground, but a locked device is refused.
	if err := newClient(&fake{locked: true}).StartApp(ctx, "ABC123", testPkg); !errors.Is(err, ErrLocked) {
		t.Fatalf("err %v", err)
	}
}

func TestContextCancelled(t *testing.T) {
	c := &Client{R: ctxFake{}, Cfg: guard.Config{Serials: []string{"ABC123"}, Packages: []string{testPkg}}}
	cctx, cancel := context.WithCancel(ctx)
	cancel()
	if err := c.Tap(cctx, "ABC123", 1, 1); !errors.Is(err, context.Canceled) {
		t.Fatalf("err %v", err)
	}
}

type ctxFake struct{}

func (ctxFake) Run(ctx context.Context, _ ...string) ([]byte, error) { return nil, ctx.Err() }

func TestRangeChecks(t *testing.T) {
	c := newClient(&fake{})
	if c.Tap(ctx, "ABC123", -1, 5) == nil || c.Swipe(ctx, "ABC123", 1, 1, 2, 2, 10) == nil {
		t.Fatal("expected range errors")
	}
}

func TestParseStateFailsClosed(t *testing.T) {
	foc := "mCurrentFocus=Window{1 u0 " + testPkg + "/" + testPkg + ".Main}\n"
	cases := []struct {
		name, in    string
		locked      bool
		pkg         string
		wantUnknown bool
	}{
		{"unlocked", "mDreamingLockscreen=false\n" + foc, false, testPkg, false},
		{"locked", "mDreamingLockscreen=true\n" + foc, true, testPkg, false},
		{"either marker true locks", "mDreamingLockscreen=false isStatusBarKeyguard=true\n" + foc, true, testPkg, false},
		{"no markers", "nothing here\n" + foc, false, "", true},
		{"empty", "", false, "", true},
		{"non-boolean value", "mDreamingLockscreen=maybe\n" + foc, false, "", true},
		{"empty value", "mDreamingLockscreen=\n" + foc, false, "", true},
		{"one bad among good", "mDreamingLockscreen=false isStatusBarKeyguard=null\n" + foc, false, "", true},
		{"conflicting focus", "mDreamingLockscreen=false\n" + foc + "mCurrentFocus=Window{2 u0 com.other.app/com.other.app.A}\n", false, "", true},
		{"repeated identical focus ok", "mDreamingLockscreen=false\n" + foc + foc, false, testPkg, false},
	}
	for _, c := range cases {
		l, p, err := parseState(c.in)
		if c.wantUnknown {
			if !errors.Is(err, ErrUnknownState) {
				t.Errorf("%s: want ErrUnknownState, got %v", c.name, err)
			}
			continue
		}
		if err != nil || l != c.locked || p != c.pkg {
			t.Errorf("%s: got locked=%v pkg=%q err=%v", c.name, l, p, err)
		}
	}
}

func TestCaptureDiscardedIfForegroundChangesDuringCapture(t *testing.T) {
	f := &fake{switchTo: "com.personal.messenger"}
	c := newClient(f)
	if b, err := c.Screenshot(ctx, "ABC123"); !errors.Is(err, ErrForeground) || b != nil {
		t.Fatalf("screenshot: data=%v err=%v", b, err)
	}
	f2 := &fake{switchTo: "com.personal.messenger"}
	if b, err := newClient(f2).UIDump(ctx, "ABC123"); !errors.Is(err, ErrForeground) || b != nil {
		t.Fatalf("ui: data=%v err=%v", b, err)
	}
}

func TestCaptureDiscardedIfLockedDuringCapture(t *testing.T) {
	f := &fake{}
	c := newClient(f)
	// Locks right after the capture command: second gate must refuse.
	c.R = lockAfterCapture{f}
	if b, err := c.Screenshot(ctx, "ABC123"); !errors.Is(err, ErrLocked) || b != nil {
		t.Fatalf("data=%v err=%v", b, err)
	}
}

type lockAfterCapture struct{ f *fake }

func (l lockAfterCapture) Run(ctx context.Context, args ...string) ([]byte, error) {
	j := strings.Join(args, " ")
	if strings.Contains(j, "screencap") {
		l.f.locked = true
	}
	return l.f.Run(ctx, args...)
}
