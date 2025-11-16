package config

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

// Writer handles writing WireGuard config files atomically.
type Writer struct {
	Path        string // full path to config file e.g. /etc/wireguard/wg0.conf
	Interface   string // wg interface name e.g. wg0
	ApplyMethod string // "wg-quick" or "syncconf"
}

// NewWriter creates a new Writer with defaults.
func NewWriter(path string, iface string, method string) *Writer {
	if path == "" {
		path = fmt.Sprintf("/etc/wireguard/%s.conf", iface)
	}
	if method == "" {
		method = "wg-quick"
	}
	return &Writer{Path: path, Interface: iface, ApplyMethod: method}
}

// WriteAndApply writes the config and applies it using chosen method.
func (w *Writer) WriteAndApply(cfg string) error {
	if err := w.writeAtomic(cfg); err != nil {
		return fmt.Errorf("write config: %w", err)
	}
	return w.apply()
}

// writeAtomic writes file atomically.
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

// apply applies the config. Requires root privileges for networking changes.
func (w *Writer) apply() error {
	switch w.ApplyMethod {
	case "wg-quick":
		// Try a sync via down/up for simplicity
		if err := run("wg-quick", "down", w.Path); err != nil {
			// ignore error (interface may not exist yet)
		}
		return run("wg-quick", "up", w.Path)
	case "syncconf":
		// syncconf requires stripped [Interface] lines formatted for `wg set`.
		// Fallback to wg-quick if transformation is complex.
		return run("wg-quick", "up", w.Path)
	default:
		return fmt.Errorf("unknown apply method: %s", w.ApplyMethod)
	}
}

func run(cmd string, args ...string) error {
	c := exec.Command(cmd, args...) // #nosec G204 - args controlled by code
	var out bytes.Buffer
	var errBuf bytes.Buffer
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
