// droidcommand: guarded local adb automation for authorised Android test devices.
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"strconv"

	"github.com/2E0LXY/DroidCommand/internal/adb"
	"github.com/2E0LXY/DroidCommand/internal/dc"
	"github.com/2E0LXY/DroidCommand/internal/guard"
	"github.com/2E0LXY/DroidCommand/internal/mcp"
)

// version is overridden at build time: -ldflags "-X main.version=X.Y.Z".
var version = "dev"

const usage = `usage: droidcommand [-config FILE] [-serial S] [-yes] <command> [args]
commands: version | doctor | serve (stdio MCP; add -allow-text to enable text entry) | devices | screenshot OUT.png | ui | tap X Y | swipe X1 Y1 X2 Y2 MS | text STRING | start-app PKG
config (JSON): {"serials":[...],"packages":[...],"audit_log":"audit.jsonl"}; empty lists deny everything.`

func main() {
	cfgPath := flag.String("config", "droidcommand.json", "config file")
	serial := flag.String("serial", "", "device serial")
	yes := flag.Bool("yes", false, "approve consequential actions (text entry)")
	allowText := flag.Bool("allow-text", false, "serve: expose the text tool (owner approval for typing)")
	flag.Parse()
	a := flag.Args()
	if len(a) == 0 {
		fmt.Fprintln(os.Stderr, usage)
		os.Exit(2)
	}
	if a[0] == "version" {
		fmt.Println("droidcommand", version)
		return
	}
	if a[0] == "doctor" {
		p, err := exec.LookPath("adb")
		if err != nil {
			fail(fmt.Errorf("adb not found in PATH"))
		}
		fmt.Println("adb:", p)
		return
	}
	cfg, err := guard.LoadConfig(*cfgPath)
	if err != nil {
		fail(err)
	}
	c := &dc.Client{R: adb.Exec{}, Cfg: cfg, Audit: guard.NewAudit(cfg.AuditLog)}
	ctx := context.Background()
	ints := func(s []string) []int {
		r := make([]int, len(s))
		for i, v := range s {
			n, e := strconv.Atoi(v)
			if e != nil {
				fail(e)
			}
			r[i] = n
		}
		return r
	}
	need := func(n int) {
		if len(a)-1 != n {
			fmt.Fprintln(os.Stderr, usage)
			os.Exit(2)
		}
	}
	switch a[0] {
	case "serve":
		ctx, stop := signal.NotifyContext(ctx, os.Interrupt)
		defer stop()
		srv := &mcp.Server{C: c, AllowText: *allowText, Version: version}
		done := make(chan error, 1)
		go func() { done <- srv.Serve(ctx, os.Stdin, os.Stdout) }()
		select {
		case e := <-done:
			check(e)
		case <-ctx.Done(): // a blocked stdin read cannot be interrupted; exit instead
		}
	case "devices":
		d, e := c.Devices(ctx)
		check(e)
		for _, s := range d {
			fmt.Println(s)
		}
	case "screenshot":
		need(1)
		b, e := c.Screenshot(ctx, *serial)
		check(e)
		check(os.WriteFile(a[1], b, 0o600))
	case "ui":
		b, e := c.UIDump(ctx, *serial)
		check(e)
		os.Stdout.Write(b)
	case "tap":
		need(2)
		n := ints(a[1:])
		check(c.Tap(ctx, *serial, n[0], n[1]))
	case "swipe":
		need(5)
		n := ints(a[1:])
		check(c.Swipe(ctx, *serial, n[0], n[1], n[2], n[3], n[4]))
	case "text":
		need(1)
		check(c.Text(ctx, *serial, a[1], *yes))
	case "start-app":
		need(1)
		check(c.StartApp(ctx, *serial, a[1]))
	default:
		fmt.Fprintln(os.Stderr, usage)
		os.Exit(2)
	}
}

func check(e error) {
	if e != nil {
		fail(e)
	}
}

func fail(e error) {
	fmt.Fprintln(os.Stderr, "error:", e)
	os.Exit(1)
}
