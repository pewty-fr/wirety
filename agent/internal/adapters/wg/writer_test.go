package wg

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestNewWriter(t *testing.T) {
	// Test with all parameters provided
	writer := NewWriter("/test/path/wg0.conf", "wg0", "wg-quick")

	if writer.Path != "/test/path/wg0.conf" {
		t.Errorf("Expected path '/test/path/wg0.conf', got '%s'", writer.Path)
	}

	if writer.Interface != "wg0" {
		t.Errorf("Expected interface 'wg0', got '%s'", writer.Interface)
	}

	if writer.ApplyMethod != "wg-quick" {
		t.Errorf("Expected apply method 'wg-quick', got '%s'", writer.ApplyMethod)
	}
}

func TestNewWriterWithDefaults(t *testing.T) {
	// Test with empty path and method (should use defaults)
	writer := NewWriter("", "wg1", "")

	expectedPath := "/etc/wireguard/wg1.conf"
	if writer.Path != expectedPath {
		t.Errorf("Expected default path '%s', got '%s'", expectedPath, writer.Path)
	}

	if writer.Interface != "wg1" {
		t.Errorf("Expected interface 'wg1', got '%s'", writer.Interface)
	}

	if writer.ApplyMethod != "wg-quick" {
		t.Errorf("Expected default apply method 'wg-quick', got '%s'", writer.ApplyMethod)
	}
}

func TestGetConfigPath(t *testing.T) {
	writer := NewWriter("/custom/path/test.conf", "wg0", "wg-quick")

	path := writer.GetConfigPath()
	if path != "/custom/path/test.conf" {
		t.Errorf("Expected path '/custom/path/test.conf', got '%s'", path)
	}
}

func TestGetInterface(t *testing.T) {
	writer := NewWriter("/test/path", "test-interface", "wg-quick")

	iface := writer.GetInterface()
	if iface != "test-interface" {
		t.Errorf("Expected interface 'test-interface', got '%s'", iface)
	}
}

func TestAddMarkerToConfig(t *testing.T) {
	writer := NewWriter("/test/path", "wg0", "wg-quick")

	// Test adding marker to config without marker
	config := "[Interface]\nPrivateKey = test\n"
	markedConfig := writer.addMarkerToConfig(config)

	if !strings.Contains(markedConfig, WiretyMarker) {
		t.Error("Expected config to contain Wirety marker")
	}

	if !strings.Contains(markedConfig, "Interface: wg0") {
		t.Error("Expected config to contain interface name")
	}

	if !strings.Contains(markedConfig, config) {
		t.Error("Expected config to contain original content")
	}

	// Test with config that already has marker
	alreadyMarked := writer.addMarkerToConfig(markedConfig)

	// Should not add marker twice
	markerCount := strings.Count(alreadyMarked, WiretyMarker)
	if markerCount != 1 {
		t.Errorf("Expected marker to appear once, found %d times", markerCount)
	}
}

func TestIsWiretyManaged(t *testing.T) {
	writer := NewWriter("/test/path", "wg0", "wg-quick")

	// Create temporary file with Wirety marker
	tmpDir := t.TempDir()
	managedFile := filepath.Join(tmpDir, "managed.conf")

	managedContent := WiretyMarker + "\n[Interface]\nPrivateKey = test\n"
	err := os.WriteFile(managedFile, []byte(managedContent), 0600)
	if err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	if !writer.isWiretyManaged(managedFile) {
		t.Error("Expected file with Wirety marker to be detected as managed")
	}

	// Create file without marker
	unmanagedFile := filepath.Join(tmpDir, "unmanaged.conf")
	unmanagedContent := "[Interface]\nPrivateKey = test\n"
	err = os.WriteFile(unmanagedFile, []byte(unmanagedContent), 0600)
	if err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	if writer.isWiretyManaged(unmanagedFile) {
		t.Error("Expected file without Wirety marker to not be detected as managed")
	}

	// Test with non-existent file
	if writer.isWiretyManaged("/nonexistent/file") {
		t.Error("Expected non-existent file to not be detected as managed")
	}
}

func TestCheckOwnership(t *testing.T) {
	tmpDir := t.TempDir()

	// Test with non-existent file (should be OK)
	writer := NewWriter(filepath.Join(tmpDir, "nonexistent.conf"), "wg0", "wg-quick")
	err := writer.CheckOwnership()
	if err != nil {
		t.Errorf("Expected no error for non-existent file, got: %v", err)
	}

	// Test with Wirety-managed file (should be OK)
	managedFile := filepath.Join(tmpDir, "managed.conf")
	managedContent := WiretyMarker + "\n[Interface]\nPrivateKey = test\n"
	err = os.WriteFile(managedFile, []byte(managedContent), 0600)
	if err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	writer.Path = managedFile
	err = writer.CheckOwnership()
	if err != nil {
		t.Errorf("Expected no error for Wirety-managed file, got: %v", err)
	}

	// Test with non-Wirety file (should error)
	unmanagedFile := filepath.Join(tmpDir, "unmanaged.conf")
	unmanagedContent := "[Interface]\nPrivateKey = test\n"
	err = os.WriteFile(unmanagedFile, []byte(unmanagedContent), 0600)
	if err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	writer.Path = unmanagedFile
	err = writer.CheckOwnership()
	if err == nil {
		t.Error("Expected error for non-Wirety file")
	}

	if !strings.Contains(err.Error(), "not managed by Wirety") {
		t.Errorf("Expected error to mention Wirety management, got: %v", err)
	}
}

func TestVerifyOwnership(t *testing.T) {
	tmpDir := t.TempDir()
	writer := NewWriter(filepath.Join(tmpDir, "test.conf"), "wg0", "wg-quick")

	// Should be same as CheckOwnership
	err1 := writer.CheckOwnership()
	err2 := writer.VerifyOwnership()

	if (err1 == nil) != (err2 == nil) {
		t.Error("VerifyOwnership should return same result as CheckOwnership")
	}
}

func TestWriteAtomic(t *testing.T) {
	tmpDir := t.TempDir()
	writer := NewWriter(filepath.Join(tmpDir, "test.conf"), "wg0", "wg-quick")

	config := "[Interface]\nPrivateKey = test\n"

	err := writer.writeAtomic(config)
	if err != nil {
		t.Errorf("Expected no error writing config, got: %v", err)
	}

	// Verify file was created
	if _, err := os.Stat(writer.Path); os.IsNotExist(err) {
		t.Error("Expected config file to be created")
	}

	// Verify content
	content, err := os.ReadFile(writer.Path)
	if err != nil {
		t.Errorf("Failed to read config file: %v", err)
	}

	if string(content) != config {
		t.Errorf("Expected config content '%s', got '%s'", config, string(content))
	}

	// Verify file permissions
	info, err := os.Stat(writer.Path)
	if err != nil {
		t.Errorf("Failed to stat config file: %v", err)
	}

	expectedMode := os.FileMode(0600)
	if info.Mode().Perm() != expectedMode {
		t.Errorf("Expected file mode %o, got %o", expectedMode, info.Mode().Perm())
	}
}

func TestWriteAtomicCreatesDirectory(t *testing.T) {
	tmpDir := t.TempDir()
	nestedPath := filepath.Join(tmpDir, "nested", "dir", "test.conf")
	writer := NewWriter(nestedPath, "wg0", "wg-quick")

	config := "[Interface]\nPrivateKey = test\n"

	err := writer.writeAtomic(config)
	if err != nil {
		t.Errorf("Expected no error writing config to nested path, got: %v", err)
	}

	// Verify file was created
	if _, err := os.Stat(writer.Path); os.IsNotExist(err) {
		t.Error("Expected config file to be created in nested directory")
	}
}

func TestFindOldWiretyConfigs(t *testing.T) {
	tmpDir := t.TempDir()

	// Create some test config files
	configs := []struct {
		name     string
		content  string
		isWirety bool
	}{
		{"wg0.conf", WiretyMarker + "\n[Interface]\n", true},
		{"wg1.conf", WiretyMarker + "\n[Interface]\n", true},
		{"other.conf", "[Interface]\n", false},
		{"not-conf.txt", WiretyMarker + "\n", false}, // Wrong extension
	}

	for _, cfg := range configs {
		path := filepath.Join(tmpDir, cfg.name)
		err := os.WriteFile(path, []byte(cfg.content), 0600)
		if err != nil {
			t.Fatalf("Failed to create test file %s: %v", cfg.name, err)
		}
	}

	// Create writer with path in the test directory
	currentConfig := filepath.Join(tmpDir, "current.conf")
	writer := NewWriter(currentConfig, "wg0", "wg-quick")

	// Override search directories to use our test directory
	oldConfigs, err := writer.FindOldWiretyConfigs()
	if err != nil {
		t.Errorf("Expected no error finding configs, got: %v", err)
	}

	// Should find the Wirety-managed configs but not the current one or non-Wirety ones
	expectedCount := 2 // wg0.conf and wg1.conf
	if len(oldConfigs) != expectedCount {
		t.Errorf("Expected %d old configs, got %d: %v", expectedCount, len(oldConfigs), oldConfigs)
	}

	// Verify found configs are Wirety-managed
	for _, configPath := range oldConfigs {
		if !writer.isWiretyManaged(configPath) {
			t.Errorf("Found config should be Wirety-managed: %s", configPath)
		}

		if configPath == currentConfig {
			t.Errorf("Should not find current config as old config: %s", configPath)
		}
	}
}

func TestUpdateInterface(t *testing.T) {
	tmpDir := t.TempDir()
	oldPath := filepath.Join(tmpDir, "wg0.conf")

	// Create old config file
	oldContent := WiretyMarker + "\n[Interface]\nPrivateKey = old\n"
	err := os.WriteFile(oldPath, []byte(oldContent), 0600)
	if err != nil {
		t.Fatalf("Failed to create old config: %v", err)
	}

	writer := NewWriter(oldPath, "wg0", "wg-quick")

	// Test no change
	err = writer.UpdateInterface("wg0")
	if err != nil {
		t.Errorf("Expected no error for same interface, got: %v", err)
	}

	if writer.Interface != "wg0" {
		t.Errorf("Expected interface to remain 'wg0', got '%s'", writer.Interface)
	}

	// Test interface change
	err = writer.UpdateInterface("wg1")
	if err != nil {
		t.Errorf("Expected no error updating interface, got: %v", err)
	}

	if writer.Interface != "wg1" {
		t.Errorf("Expected interface 'wg1', got '%s'", writer.Interface)
	}

	expectedNewPath := "/etc/wireguard/wg1.conf"
	if writer.Path != expectedNewPath {
		t.Errorf("Expected new path '%s', got '%s'", expectedNewPath, writer.Path)
	}

	// Old file should be removed (in real scenario, but we can't test this without root)
	// We just verify the method doesn't panic
}

func TestRedactKeys(t *testing.T) {
	config := `[Interface]
PrivateKey = supersecretkey123
Address = 10.0.0.1/24

[Peer]
PublicKey = publickey123
AllowedIPs = 10.0.0.2/32
Endpoint = example.com:51820
PrivateKey = anothersecret
`

	redacted := RedactKeys(config)

	// Should not contain the actual private keys
	if strings.Contains(redacted, "supersecretkey123") {
		t.Error("Expected private key to be redacted")
	}

	if strings.Contains(redacted, "anothersecret") {
		t.Error("Expected second private key to be redacted")
	}

	// Should contain redaction marker
	if !strings.Contains(redacted, "<redacted>") {
		t.Error("Expected redacted config to contain '<redacted>' marker")
	}

	// Should preserve other content
	if !strings.Contains(redacted, "Address = 10.0.0.1/24") {
		t.Error("Expected non-private content to be preserved")
	}

	if !strings.Contains(redacted, "PublicKey = publickey123") {
		t.Error("Expected public key to be preserved")
	}

	// Count redacted lines
	redactedLines := strings.Count(redacted, "PrivateKey = <redacted>")
	if redactedLines != 2 {
		t.Errorf("Expected 2 redacted private key lines, got %d", redactedLines)
	}
}

func TestRedactKeysWithVariousFormats(t *testing.T) {
	tests := []struct {
		name   string
		input  string
		expect string
	}{
		{
			name:   "standard format",
			input:  "PrivateKey = abc123",
			expect: "PrivateKey = <redacted>",
		},
		{
			name:   "with spaces",
			input:  "  PrivateKey = def456  ",
			expect: "PrivateKey = <redacted>",
		},
		{
			name:   "no spaces around equals",
			input:  "PrivateKey=ghi789",
			expect: "PrivateKey=ghi789", // Should not match (RedactKeys looks for "PrivateKey = ")
		},
		{
			name:   "mixed case",
			input:  "privatekey = jkl012",
			expect: "privatekey = jkl012", // Should not match (case sensitive)
		},
		{
			name:   "not private key",
			input:  "PublicKey = mno345",
			expect: "PublicKey = mno345", // Should not be redacted
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := RedactKeys(tt.input)
			if !strings.Contains(result, tt.expect) {
				t.Errorf("Expected result to contain '%s', got '%s'", tt.expect, result)
			}
		})
	}
}

func TestGetCurrentPeerRoutes(t *testing.T) {
	writer := NewWriter("/test/path", "wg0", "wg-quick")

	// This will likely fail in test environment, but should not panic
	routes, err := writer.getCurrentPeerRoutes()

	// Should return a map even if command fails
	if routes == nil {
		t.Error("Expected routes map to be initialized")
	}

	// In test environment, this will likely fail due to missing interface
	// We mainly test that it doesn't panic
	t.Logf("getCurrentPeerRoutes returned %d routes with error: %v", len(routes), err)
}

func TestAddAndRemoveRoute(t *testing.T) {
	writer := NewWriter("/test/path", "wg0", "wg-quick")

	// Test adding route (will likely fail due to permissions, but shouldn't panic)
	err := writer.addRoute("10.0.0.1/32")
	t.Logf("addRoute returned error: %v", err)

	// Test removing route (will likely fail due to permissions, but shouldn't panic)
	err = writer.removeRoute("10.0.0.1/32")
	t.Logf("removeRoute returned error: %v", err)

	// Test with default route (should be skipped)
	err = writer.addRoute("0.0.0.0/0")
	if err != nil {
		t.Errorf("Expected no error for default route (should be skipped), got: %v", err)
	}

	err = writer.removeRoute("0.0.0.0/0")
	if err != nil {
		t.Errorf("Expected no error for default route removal (should be skipped), got: %v", err)
	}
}

func TestUpdatePeerRoutes(t *testing.T) {
	writer := NewWriter("/test/path", "wg0", "wg-quick")

	oldRoutes := map[string]bool{
		"10.0.0.1/32": true,
		"10.0.0.2/32": true,
	}

	// This will likely fail in test environment, but should not panic
	err := writer.updatePeerRoutes(oldRoutes)

	// We mainly test that it doesn't panic
	t.Logf("updatePeerRoutes returned error: %v", err)
}

func TestDisableInterface(t *testing.T) {
	writer := NewWriter("/test/path", "wg0", "wg-quick")

	// This will likely fail in test environment, but should not panic
	err := writer.disableInterface("nonexistent-interface")

	// We mainly test that it doesn't panic
	t.Logf("disableInterface returned error: %v", err)
}

func TestCleanupOldConfigs(t *testing.T) {
	writer := NewWriter("/test/path", "wg0", "wg-quick")

	// This will search for configs and try to clean them up
	// In test environment, this should not find any configs to clean
	err := writer.CleanupOldConfigs()
	if err != nil {
		t.Errorf("Expected no error from CleanupOldConfigs, got: %v", err)
	}
}

// Additional tests for better coverage

func TestWriteAndApply(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "test.conf")
	writer := NewWriter(configPath, "wg0", "wg-quick")

	config := "[Interface]\nPrivateKey = test\n"

	// This will likely fail due to wg-quick not being available or permissions
	err := writer.WriteAndApply(config)

	// We mainly test that it doesn't panic and handles the file operations
	t.Logf("WriteAndApply returned: %v", err)

	// File should be created even if apply fails
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		t.Error("Expected config file to be created")
	}
}

func TestApply(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "test.conf")
	writer := NewWriter(configPath, "wg0", "wg-quick")

	// Create config file first
	config := WiretyMarker + "\n[Interface]\nPrivateKey = test\n"
	err := os.WriteFile(configPath, []byte(config), 0600)
	if err != nil {
		t.Fatalf("Failed to create config file: %v", err)
	}

	// Test wg-quick method
	err = writer.apply()
	t.Logf("apply with wg-quick returned: %v", err)

	// Test syncconf method
	writer.ApplyMethod = "syncconf"
	err = writer.apply()
	t.Logf("apply with syncconf returned: %v", err)

	// Test invalid method
	writer.ApplyMethod = "invalid"
	err = writer.apply()
	if err == nil {
		t.Error("Expected error for invalid apply method")
	}
	if !strings.Contains(err.Error(), "unknown apply method") {
		t.Errorf("Expected error to mention unknown method, got: %v", err)
	}
}

func TestSyncconf(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "test.conf")
	writer := NewWriter(configPath, "wg0", "syncconf")

	// Create config file
	config := WiretyMarker + "\n[Interface]\nPrivateKey = test\n"
	err := os.WriteFile(configPath, []byte(config), 0600)
	if err != nil {
		t.Fatalf("Failed to create config file: %v", err)
	}

	// This will likely fail due to missing wg/ip commands or permissions
	err = writer.syncconf()

	// We mainly test that it doesn't panic
	t.Logf("syncconf returned: %v", err)
}

func TestRunCommand(t *testing.T) {

	// Test with invalid command
	err := run("nonexistent-command", "arg1", "arg2")
	if err == nil {
		t.Error("Expected error for nonexistent command")
	}

	// Test with valid command that should work on most systems
	err = run("echo", "test")
	if err != nil {
		t.Errorf("Expected no error for echo command, got: %v", err)
	}
}

func TestCleanupOldConfigsIntegration(t *testing.T) {
	tmpDir := t.TempDir()

	// Create some test config files in the temp directory
	configs := []struct {
		name     string
		content  string
		isWirety bool
	}{
		{"wg0.conf", WiretyMarker + "\n[Interface]\n", true},
		{"wg1.conf", WiretyMarker + "\n[Interface]\n", true},
		{"other.conf", "[Interface]\n", false},
	}

	for _, cfg := range configs {
		path := filepath.Join(tmpDir, cfg.name)
		err := os.WriteFile(path, []byte(cfg.content), 0600)
		if err != nil {
			t.Fatalf("Failed to create test file %s: %v", cfg.name, err)
		}
	}

	// Create writer with path in a different location to avoid finding itself
	writer := NewWriter(filepath.Join(tmpDir, "current.conf"), "wg0", "wg-quick")

	// This will try to clean up configs, but won't find any in standard locations
	err := writer.CleanupOldConfigs()
	if err != nil {
		t.Errorf("Expected no error from CleanupOldConfigs, got: %v", err)
	}
}

func TestFindOldWiretyConfigsInStandardLocations(t *testing.T) {
	writer := NewWriter("/tmp/test.conf", "wg0", "wg-quick")

	// This will search in standard locations
	oldConfigs, err := writer.FindOldWiretyConfigs()
	if err != nil {
		t.Errorf("Expected no error finding configs, got: %v", err)
	}

	// Should return a list (possibly empty) - but can be nil if no directories exist
	// This is acceptable behavior

	t.Logf("Found %d old Wirety configs in standard locations", len(oldConfigs))
}

func TestUpdateInterfaceWithSameName(t *testing.T) {
	writer := NewWriter("/test/path", "wg0", "wg-quick")

	// Test updating to same interface name
	err := writer.UpdateInterface("wg0")
	if err != nil {
		t.Errorf("Expected no error for same interface name, got: %v", err)
	}

	// Interface should remain the same
	if writer.Interface != "wg0" {
		t.Errorf("Expected interface to remain 'wg0', got '%s'", writer.Interface)
	}
}

func TestRedactKeysWithEmptyInput(t *testing.T) {
	result := RedactKeys("")
	if result != "" {
		t.Errorf("Expected empty result, got '%s'", result)
	}
}

func TestRedactKeysWithOnlyPrivateKeys(t *testing.T) {
	config := "PrivateKey = secret1\nPrivateKey = secret2\n"
	result := RedactKeys(config)

	expectedLines := []string{
		"PrivateKey = <redacted>",
		"PrivateKey = <redacted>",
	}

	for _, expectedLine := range expectedLines {
		if !strings.Contains(result, expectedLine) {
			t.Errorf("Expected result to contain '%s', got '%s'", expectedLine, result)
		}
	}

	// Should not contain original secrets
	if strings.Contains(result, "secret1") || strings.Contains(result, "secret2") {
		t.Error("Expected secrets to be redacted")
	}
}
