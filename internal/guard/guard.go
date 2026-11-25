// Package guard holds allowlists, input validation and the audit log.
package guard

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"regexp"
	"sync"
	"time"
)

type Config struct {
	Serials  []string `json:"serials"`  // allowed device serials; empty denies all
	Packages []string `json:"packages"` // allowed package ids; empty denies all
	AuditLog string   `json:"audit_log"`
}

func LoadConfig(path string) (Config, error) {
	var c Config
	b, err := os.ReadFile(path)
	if err != nil {
		return c, err
	}
	err = json.Unmarshal(b, &c)
	return c, err
}

var (
	ErrSerial  = errors.New("device serial not in allowlist")
	ErrPackage = errors.New("package not in allowlist")
	ErrText    = errors.New("text contains characters outside [A-Za-z0-9 .,@_-]")
	ErrNeedOK  = errors.New("consequential action requires explicit approval (--yes)")

	serialRe  = regexp.MustCompile(`^[A-Za-z0-9._:-]{1,64}$`)
	packageRe = regexp.MustCompile(`^[A-Za-z][A-Za-z0-9_]*(\.[A-Za-z][A-Za-z0-9_]*)+$`)
	textRe    = regexp.MustCompile(`^[A-Za-z0-9 .,@_-]{0,256}$`)
)

func contains(l []string, s string) bool {
	for _, v := range l {
		if v == s {
			return true
		}
	}
	return false
}

func (c Config) CheckSerial(s string) error {
	if !serialRe.MatchString(s) || !contains(c.Serials, s) {
		return ErrSerial
	}
	return nil
}

func (c Config) CheckPackage(p string) error {
	if !packageRe.MatchString(p) || !contains(c.Packages, p) {
		return ErrPackage
	}
	return nil
}

// CheckText validates text for `input text`. Spaces are later encoded as %s.
func CheckText(s string) error {
	if !textRe.MatchString(s) {
		return ErrText
	}
	return nil
}

func RequireApproval(approved bool) error {
	if !approved {
		return ErrNeedOK
	}
	return nil
}

// Audit appends one JSON line per operation. Text payloads are never logged.
type Audit struct {
	mu   sync.Mutex
	path string
}

func NewAudit(path string) *Audit { return &Audit{path: path} }

func (a *Audit) Log(op, serial, target string, err error) {
	if a == nil || a.path == "" {
		return
	}
	rec := map[string]string{"t": time.Now().UTC().Format(time.RFC3339), "op": op, "serial": serial, "target": target, "ok": "true"}
	if err != nil {
		rec["ok"], rec["err"] = "false", err.Error()
	}
	b, _ := json.Marshal(rec)
	a.mu.Lock()
	defer a.mu.Unlock()
	f, e := os.OpenFile(a.path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600)
	if e != nil {
		fmt.Fprintln(os.Stderr, "audit:", e)
		return
	}
	defer f.Close()
	f.Write(append(b, '\n'))
}
