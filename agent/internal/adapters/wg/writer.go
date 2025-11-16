package wg

import (
	"bufio"
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// Writer handles writing WireGuard config files atomically and applying them.
type Writer struct {
	Path        string
	Interface   string
	ApplyMethod string
}

func NewWriter(path, iface, method string) *Writer {
	if path == "" {
		path = fmt.Sprintf("/etc/wireguard/%s.conf", iface)
	}
	if method == "" {
		method = "wg-quick"
	}
	return &Writer{Path: path, Interface: iface, ApplyMethod: method}
}

func (w *Writer) WriteAndApply(cfg string) error {
	if err := w.writeAtomic(cfg); err != nil {
		return fmt.Errorf("write config: %w", err)
	}
	return w.apply()
}

func (w *Writer) writeAtomic(cfg string) error {
	dir := filepath.Dir(w.Path)
	if err := os.MkdirAll(dir, 0o750); err != nil {
		return err
	}
	tmp := fmt.Sprintf("%s.tmp-%d", w.Path, time.Now().UnixNano())
	if err := os.WriteFile(tmp, []byte(cfg), 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, w.Path)
}

func (w *Writer) apply() error {
	switch w.ApplyMethod {
	case "wg-quick":
		_ = run("wg-quick", "down", w.Path) // ignore error
		return run("wg-quick", "up", w.Path)
	case "syncconf":
		return run("wg-quick", "up", w.Path)
	default:
		return fmt.Errorf("unknown apply method: %s", w.ApplyMethod)
	}
}

func run(cmd string, args ...string) error {
	c := exec.Command(cmd, args...) // #nosec G204
	var out, errBuf bytes.Buffer
	c.Stdout = &out
	c.Stderr = &errBuf
	if err := c.Run(); err != nil {
		return fmt.Errorf("%s %v failed: %v stderr=%s", cmd, args, err, errBuf.String())
	}
	return nil
}

// RedactKeys redacts PrivateKey values for logging.
func RedactKeys(cfg string) string {
	scanner := bufio.NewScanner(strings.NewReader(cfg))
	var b strings.Builder
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(strings.TrimSpace(line), "PrivateKey =") {
			b.WriteString("PrivateKey = <redacted>\n")
			continue
		}
		b.WriteString(line + "\n")
	}
	return b.String()
}
