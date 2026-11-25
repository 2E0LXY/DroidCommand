// Package dc implements the guarded device operations.
package dc

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/2E0LXY/DroidCommand/internal/adb"
	"github.com/2E0LXY/DroidCommand/internal/guard"
)

var ErrLocked = errors.New("device is locked; unlock it yourself, no bypass is provided")

type Client struct {
	R     adb.Runner
	Cfg   guard.Config
	Audit *guard.Audit
}

func (c *Client) dev(serial string) error { return c.Cfg.CheckSerial(serial) }

func (c *Client) shell(ctx context.Context, serial string, args ...string) ([]byte, error) {
	return c.R.Run(ctx, append([]string{"-s", serial, "shell"}, args...)...)
}

// Devices lists attached devices; only allowlisted serials in state "device" are returned.
func (c *Client) Devices(ctx context.Context) ([]string, error) {
	out, err := c.R.Run(ctx, "devices")
	if err != nil {
		return nil, err
	}
	var res []string
	for _, ln := range strings.Split(string(out), "\n")[1:] {
		f := strings.Fields(ln)
		if len(f) >= 2 && f[1] == "device" && c.Cfg.CheckSerial(f[0]) == nil {
			res = append(res, f[0])
		}
	}
	return res, nil
}

func (c *Client) locked(ctx context.Context, serial string) (bool, error) {
	out, err := c.shell(ctx, serial, "dumpsys", "window")
	if err != nil {
		return false, err
	}
	s := string(out)
	return strings.Contains(s, "mDreamingLockscreen=true") || strings.Contains(s, "isStatusBarKeyguard=true"), nil
}

// gate runs the common checks for any device-affecting operation.
func (c *Client) gate(ctx context.Context, serial string) error {
	if err := c.dev(serial); err != nil {
		return err
	}
	l, err := c.locked(ctx, serial)
	if err != nil {
		return err
	}
	if l {
		return ErrLocked
	}
	return nil
}

func (c *Client) done(op, serial, target string, err error) error {
	c.Audit.Log(op, serial, target, err)
	return err
}

func (c *Client) Screenshot(ctx context.Context, serial string) ([]byte, error) {
	if err := c.gate(ctx, serial); err != nil {
		return nil, c.done("screenshot", serial, "", err)
	}
	b, err := c.R.Run(ctx, "-s", serial, "exec-out", "screencap", "-p")
	return b, c.done("screenshot", serial, "", err)
}

func (c *Client) UIDump(ctx context.Context, serial string) ([]byte, error) {
	if err := c.gate(ctx, serial); err != nil {
		return nil, c.done("ui", serial, "", err)
	}
	b, err := c.R.Run(ctx, "-s", serial, "exec-out", "uiautomator", "dump", "/dev/tty")
	return b, c.done("ui", serial, "", err)
}

func (c *Client) Tap(ctx context.Context, serial string, x, y int) error {
	if x < 0 || y < 0 || x > 10000 || y > 10000 {
		return errors.New("coordinates out of range")
	}
	if err := c.gate(ctx, serial); err != nil {
		return c.done("tap", serial, "", err)
	}
	_, err := c.shell(ctx, serial, "input", "tap", strconv.Itoa(x), strconv.Itoa(y))
	return c.done("tap", serial, fmt.Sprintf("%d,%d", x, y), err)
}

func (c *Client) Swipe(ctx context.Context, serial string, x1, y1, x2, y2, ms int) error {
	for _, v := range []int{x1, y1, x2, y2} {
		if v < 0 || v > 10000 {
			return errors.New("coordinates out of range")
		}
	}
	if ms < 50 || ms > 5000 {
		return errors.New("duration out of range")
	}
	if err := c.gate(ctx, serial); err != nil {
		return c.done("swipe", serial, "", err)
	}
	_, err := c.shell(ctx, serial, "input", "swipe", strconv.Itoa(x1), strconv.Itoa(y1), strconv.Itoa(x2), strconv.Itoa(y2), strconv.Itoa(ms))
	return c.done("swipe", serial, "", err)
}

// Text types restricted text; consequential, so it needs approval. Payload is not audited.
func (c *Client) Text(ctx context.Context, serial, text string, approved bool) error {
	if err := guard.RequireApproval(approved); err != nil {
		return c.done("text", serial, "", err)
	}
	if err := guard.CheckText(text); err != nil {
		return c.done("text", serial, "", err)
	}
	if err := c.gate(ctx, serial); err != nil {
		return c.done("text", serial, "", err)
	}
	_, err := c.shell(ctx, serial, "input", "text", strings.ReplaceAll(text, " ", "%s"))
	return c.done("text", serial, fmt.Sprintf("%d chars", len(text)), err)
}

// StartApp launches an allowlisted package via its launcher intent.
func (c *Client) StartApp(ctx context.Context, serial, pkg string) error {
	if err := c.Cfg.CheckPackage(pkg); err != nil {
		return c.done("start-app", serial, pkg, err)
	}
	if err := c.gate(ctx, serial); err != nil {
		return c.done("start-app", serial, pkg, err)
	}
	_, err := c.shell(ctx, serial, "monkey", "-p", pkg, "-c", "android.intent.category.LAUNCHER", "1")
	return c.done("start-app", serial, pkg, err)
}
