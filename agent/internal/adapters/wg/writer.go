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

	"github.com/rs/zerolog/log"
)

const (
	// WiretyMarker is the comment added to the beginning of Wirety-managed configuration files
	WiretyMarker = "# This file is managed by Wirety Agent - DO NOT EDIT MANUALLY"
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

// CheckOwnership verifies that the target config file is managed by Wirety.
// Returns true if the file doesn't exist (new file) or contains the Wirety marker.
// Returns false if the file exists but doesn't contain the marker (not safe to overwrite).
func (w *Writer) CheckOwnership() error {
	// Check if file exists
	if _, err := os.Stat(w.Path); os.IsNotExist(err) {
		// File doesn't exist, safe to create
		return nil
	} else if err != nil {
		return fmt.Errorf("failed to stat config file %s: %w", w.Path, err)
	}

	// File exists, check if it contains our marker
	content, err := os.ReadFile(w.Path)
	if err != nil {
		return fmt.Errorf("failed to read config file %s: %w", w.Path, err)
	}

	if !strings.Contains(string(content), WiretyMarker) {
		return fmt.Errorf("config file %s exists but is not managed by Wirety (missing marker).\n"+
			"This safety check prevents overwriting existing WireGuard configurations.\n"+
			"To fix this:\n"+
			"  1. Backup your existing config: sudo cp %s %s.backup\n"+
			"  2. Remove the existing file: sudo rm %s\n"+
			"  3. Or choose a different config path with -config flag",
			w.Path, w.Path, w.Path, w.Path)
	}

	return nil
}

// VerifyOwnership is a public method to explicitly check ownership without writing
func (w *Writer) VerifyOwnership() error {
	return w.CheckOwnership()
}

// GetConfigPath returns the full path to the config file
func (w *Writer) GetConfigPath() string {
	return w.Path
}

// addMarkerToConfig ensures the configuration starts with the Wirety marker
func (w *Writer) addMarkerToConfig(cfg string) string {
	// Check if marker is already present
	if strings.Contains(cfg, WiretyMarker) {
		return cfg
	}

	// Add marker at the beginning with timestamp
	timestamp := time.Now().Format(time.RFC3339)
	header := fmt.Sprintf("%s\n# Generated on: %s\n# Interface: %s\n\n", WiretyMarker, timestamp, w.Interface)

	return header + cfg
}

func (w *Writer) WriteAndApply(cfg string) error {
	// First, check if we own this config file
	if err := w.CheckOwnership(); err != nil {
		return fmt.Errorf("ownership check failed: %w", err)
	}

	// Add marker to config
	markedConfig := w.addMarkerToConfig(cfg)

	if err := w.writeAtomic(markedConfig); err != nil {
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
		// Use wg syncconf with wg-quick strip to update config without recreating interface
		// This is equivalent to: wg syncconf <interface> <(wg-quick strip <config>)
		return w.syncconf()
	default:
		return fmt.Errorf("unknown apply method: %s", w.ApplyMethod)
	}
}

// syncconf applies configuration using wg syncconf with wg-quick strip
// This updates the interface without bringing it down
func (w *Writer) syncconf() error {
	// First, ensure the interface exists (create it if needed)
	// Check if interface exists
	checkCmd := exec.Command("ip", "link", "show", w.Interface) // #nosec G204 - w.Interface is sanitized and controlled
	if err := checkCmd.Run(); err != nil {
		// Interface doesn't exist, create it with wg-quick up
		log.Info().Str("interface", w.Interface).Msg("interface doesn't exist, creating with wg-quick up")
		if err := run("wg-quick", "up", w.Path); err != nil {
			return fmt.Errorf("failed to create interface: %w", err)
		}
		return nil
	}

	// Interface exists, use syncconf to update it
	// Run: wg-quick strip <config> | wg syncconf <interface> /dev/stdin
	stripCmd := exec.Command("wg-quick", "strip", w.Path)                // #nosec G204 - w.Path is controlled by agent
	syncCmd := exec.Command("wg", "syncconf", w.Interface, "/dev/stdin") // #nosec G204 - w.Interface is sanitized and controlled

	// Pipe strip output to syncconf input
	pipe, err := stripCmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("failed to create pipe: %w", err)
	}
	syncCmd.Stdin = pipe

	// Capture errors
	var stripErr, syncErr bytes.Buffer
	stripCmd.Stderr = &stripErr
	syncCmd.Stderr = &syncErr

	// Start both commands
	if err := stripCmd.Start(); err != nil {
		return fmt.Errorf("failed to start wg-quick strip: %w", err)
	}
	if err := syncCmd.Start(); err != nil {
		return fmt.Errorf("failed to start wg syncconf: %w", err)
	}

	// Wait for both to complete
	if err := stripCmd.Wait(); err != nil {
		return fmt.Errorf("wg-quick strip failed: %v stderr=%s", err, stripErr.String())
	}
	if err := syncCmd.Wait(); err != nil {
		return fmt.Errorf("wg syncconf failed: %v stderr=%s", err, syncErr.String())
	}

	log.Debug().Str("interface", w.Interface).Msg("configuration synced successfully")
	return nil
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

// FindOldWiretyConfigs searches for other Wirety-managed configuration files
// in common WireGuard locations and returns their paths and interface names.
func (w *Writer) FindOldWiretyConfigs() ([]string, error) {
	var oldConfigs []string

	// Common WireGuard config directories
	searchDirs := []string{
		"/etc/wireguard",
		"/opt/wireguard",
		".", // Current directory for testing
	}

	for _, dir := range searchDirs {
		// Skip if directory doesn't exist
		if _, err := os.Stat(dir); os.IsNotExist(err) {
			continue
		}

		entries, err := os.ReadDir(dir)
		if err != nil {
			continue // Skip directories we can't read
		}

		for _, entry := range entries {
			if entry.IsDir() {
				continue
			}

			// Look for .conf files
			if !strings.HasSuffix(entry.Name(), ".conf") {
				continue
			}

			configPath := filepath.Join(dir, entry.Name())

			// Skip the current config file
			if configPath == w.Path {
				continue
			}

			// Check if this config file is managed by Wirety
			if w.isWiretyManaged(configPath) {
				oldConfigs = append(oldConfigs, configPath)
			}
		}
	}

	return oldConfigs, nil
}

// isWiretyManaged checks if a config file contains the Wirety marker
func (w *Writer) isWiretyManaged(configPath string) bool {
	content, err := os.ReadFile(configPath) // #nosec G304 - configPath is from controlled directory scan
	if err != nil {
		return false
	}
	return strings.Contains(string(content), WiretyMarker)
}

// CleanupOldConfigs removes old Wirety-managed configs and disables their interfaces
func (w *Writer) CleanupOldConfigs() error {
	oldConfigs, err := w.FindOldWiretyConfigs()
	if err != nil {
		return fmt.Errorf("failed to find old configs: %w", err)
	}

	for _, configPath := range oldConfigs {
		// Extract interface name from config path
		basename := filepath.Base(configPath)
		ifaceName := strings.TrimSuffix(basename, ".conf")

		fmt.Printf("Found old Wirety config: %s (interface: %s)\n", configPath, ifaceName)

		// Try to bring down the interface
		if err := w.disableInterface(ifaceName); err != nil {
			fmt.Printf("Warning: failed to disable interface %s: %v\n", ifaceName, err)
			// Continue anyway - we still want to remove the config
		}

		// Remove the config file
		if err := os.Remove(configPath); err != nil {
			fmt.Printf("Warning: failed to remove config %s: %v\n", configPath, err)
		} else {
			fmt.Printf("Removed old Wirety config: %s\n", configPath)
		}
	}

	return nil
}

// disableInterface attempts to bring down a WireGuard interface
func (w *Writer) disableInterface(ifaceName string) error {
	// First try wg-quick down
	cmd := exec.Command("wg-quick", "down", ifaceName)
	if err := cmd.Run(); err != nil {
		// If wg-quick fails, try removing the interface directly
		cmd = exec.Command("ip", "link", "delete", ifaceName)
		return cmd.Run()
	}
	return nil
}

// GetInterface returns the interface name
func (w *Writer) GetInterface() string {
	return w.Interface
}

// UpdateInterface changes the interface name and updates the config path accordingly
// This also handles cleaning up the old interface and config file
func (w *Writer) UpdateInterface(newInterface string) error {
	if newInterface == w.Interface {
		return nil // No change needed
	}

	oldInterface := w.Interface
	oldPath := w.Path

	log.Info().
		Str("old_interface", oldInterface).
		Str("new_interface", newInterface).
		Str("old_path", oldPath).
		Msg("updating interface name")

	// Calculate new config path if using default pattern
	newPath := w.Path
	if w.Path == "" || strings.Contains(w.Path, "/"+oldInterface+".conf") {
		newPath = fmt.Sprintf("/etc/wireguard/%s.conf", newInterface)
	}

	// Try to bring down old interface
	if err := w.disableInterface(oldInterface); err != nil {
		log.Warn().Err(err).Str("interface", oldInterface).Msg("failed to disable old interface")
	}

	// Remove old config file if it's Wirety-managed
	if w.isWiretyManaged(oldPath) {
		if err := os.Remove(oldPath); err != nil {
			log.Warn().Err(err).Str("path", oldPath).Msg("failed to remove old config file")
		} else {
			log.Info().Str("path", oldPath).Msg("removed old config file")
		}
	}

	// Update writer configuration
	w.Interface = newInterface
	w.Path = newPath

	log.Info().
		Str("interface", newInterface).
		Str("path", newPath).
		Msg("interface update completed")

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
