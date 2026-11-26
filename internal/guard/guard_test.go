package guard

import (
	"errors"
	"os"
	"strings"
	"testing"
)

func TestEmptyAllowlistsDenyAll(t *testing.T) {
	var c Config
	if !errors.Is(c.CheckSerial("ABC123"), ErrSerial) || !errors.Is(c.CheckPackage("a.b"), ErrPackage) {
		t.Fatal("empty allowlists must deny")
	}
}

func TestMalformedIdentifiersDenied(t *testing.T) {
	c := Config{Serials: []string{"a b", "ok1"}, Packages: []string{"nodots", "a.b"}}
	if c.CheckSerial("a b") == nil || c.CheckPackage("nodots") == nil {
		t.Fatal("malformed identifiers must be denied even if allowlisted")
	}
	if c.CheckSerial("ok1") != nil || c.CheckPackage("a.b") != nil {
		t.Fatal("valid allowlisted identifiers must pass")
	}
}

func TestCheckText(t *testing.T) {
	for _, bad := range []string{"a;b", "a$(x)", "a\nb", "a'b", "a|b", "a`b", strings.Repeat("a", 257)} {
		if CheckText(bad) == nil {
			t.Errorf("%q must be rejected", bad)
		}
	}
	if CheckText("user@example.com 1_2-3.") != nil {
		t.Fatal("safe text rejected")
	}
}

func TestAuditAppendsJSONLAndHandlesNil(t *testing.T) {
	var nilAudit *Audit
	nilAudit.Log("x", "s", "t", nil) // must not panic
	p := t.TempDir() + "/a.jsonl"
	a := NewAudit(p)
	a.Log("tap", "S", "1,2", nil)
	a.Log("tap", "S", "", errors.New("denied"))
	b, _ := os.ReadFile(p)
	if n := strings.Count(string(b), "\n"); n != 2 || !strings.Contains(string(b), `"err":"denied"`) {
		t.Fatalf("audit: %s", b)
	}
	if st, _ := os.Stat(p); st.Mode().Perm() != 0o600 {
		t.Fatalf("audit mode %v", st.Mode())
	}
}
