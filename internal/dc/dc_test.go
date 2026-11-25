package dc

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/2E0LXY/DroidCommand/internal/guard"
)

type fake struct {
	calls  [][]string
	locked bool
}

func (f *fake) Run(_ context.Context, args ...string) ([]byte, error) {
	f.calls = append(f.calls, args)
	j := strings.Join(args, " ")
	switch {
	case j == "devices":
		return []byte("List of devices attached\nABC123\tdevice\nEVIL9\tdevice\nOFF1\toffline\n"), nil
	case strings.Contains(j, "dumpsys window"):
		if f.locked {
			return []byte("mDreamingLockscreen=true"), nil
		}
		return []byte("mDreamingLockscreen=false"), nil
	}
	return []byte("ok"), nil
}

func newClient(f *fake) *Client {
	return &Client{R: f, Cfg: guard.Config{Serials: []string{"ABC123"}, Packages: []string{"uk.co.droidcommand.testapp"}}}
}

func TestDevicesFiltersAllowlistAndState(t *testing.T) {
	got, _ := newClient(&fake{}).Devices(context.Background())
	if len(got) != 1 || got[0] != "ABC123" {
		t.Fatalf("got %v", got)
	}
}

func TestSerialDenied(t *testing.T) {
	f := &fake{}
	if err := newClient(f).Tap(context.Background(), "EVIL9", 1, 1); !errors.Is(err, guard.ErrSerial) {
		t.Fatalf("err %v", err)
	}
	if len(f.calls) != 0 {
		t.Fatal("adb must not be called for denied serial")
	}
}

func TestLockedRefused(t *testing.T) {
	f := &fake{locked: true}
	if err := newClient(f).Tap(context.Background(), "ABC123", 1, 1); !errors.Is(err, ErrLocked) {
		t.Fatalf("err %v", err)
	}
	for _, c := range f.calls {
		if strings.Contains(strings.Join(c, " "), "input") {
			t.Fatal("input sent to locked device")
		}
	}
}

func TestTextNeedsApprovalAndCharset(t *testing.T) {
	c := newClient(&fake{})
	if err := c.Text(context.Background(), "ABC123", "hi", false); !errors.Is(err, guard.ErrNeedOK) {
		t.Fatalf("err %v", err)
	}
	if err := c.Text(context.Background(), "ABC123", "a;reboot", true); !errors.Is(err, guard.ErrText) {
		t.Fatalf("err %v", err)
	}
	if err := c.Text(context.Background(), "ABC123", "hello world", true); err != nil {
		t.Fatal(err)
	}
}

func TestStartAppAllowlist(t *testing.T) {
	c := newClient(&fake{})
	if err := c.StartApp(context.Background(), "ABC123", "com.bank.app"); !errors.Is(err, guard.ErrPackage) {
		t.Fatalf("err %v", err)
	}
	if err := c.StartApp(context.Background(), "ABC123", "uk.co.droidcommand.testapp"); err != nil {
		t.Fatal(err)
	}
}

func TestRangeChecks(t *testing.T) {
	c := newClient(&fake{})
	if c.Tap(context.Background(), "ABC123", -1, 5) == nil || c.Swipe(context.Background(), "ABC123", 1, 1, 2, 2, 10) == nil {
		t.Fatal("expected range errors")
	}
}
