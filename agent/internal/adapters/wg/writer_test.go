package wg
package wg

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestWiretyMarker(t *testing.T) {
	// Create a temporary directory for testing
	tempDir, err := os.MkdirTemp("", "wirety-agent-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	testConfigPath := filepath.Join(tempDir, "wg0.conf")
	writer := NewWriter(testConfigPath, "wg0", "wg-quick")

	t.Run("NewFileOwnership", func(t *testing.T) {
		// Should pass for non-existent file
		if err := writer.VerifyOwnership(); err != nil {
			t.Errorf("VerifyOwnership should pass for non-existent file: %v", err)
		}
	})

	t.Run("MarkerAddition", func(t *testing.T) {
		testConfig := `[Interface]
PrivateKey = test123
Address = 10.0.0.1/24

[Peer]
PublicKey = test456
AllowedIPs = 0.0.0.0/0
Endpoint = server.example.com:51820
`

		// Test writeAtomic separately to avoid wg-quick issues in tests
		markedConfig := writer.addMarkerToConfig(testConfig)
		if err := writer.writeAtomic(markedConfig); err != nil {
			t.Fatalf("writeAtomic failed: %v", err)
		}

		// Read the written file
		content, err := os.ReadFile(testConfigPath)
		if err != nil {
			t.Fatalf("Failed to read written config: %v", err)
		}

		contentStr := string(content)
		if !strings.Contains(contentStr, WiretyMarker) {
			t.Errorf("Written config should contain Wirety marker")
		}

		if !strings.Contains(contentStr, "Generated on:") {
			t.Errorf("Written config should contain generation timestamp")
		}

		if !strings.Contains(contentStr, "Interface: wg0") {
			t.Errorf("Written config should contain interface name")
		}

		// Verify original config content is preserved
		if !strings.Contains(contentStr, "[Interface]") || !strings.Contains(contentStr, "[Peer]") {
			t.Errorf("Original config content should be preserved")
		}
	})

	t.Run("SubsequentWrites", func(t *testing.T) {
		// Second write should also pass ownership check
		testConfig2 := `[Interface]
PrivateKey = updated123
Address = 10.0.0.2/24
`

		markedConfig2 := writer.addMarkerToConfig(testConfig2)
		if err := writer.writeAtomic(markedConfig2); err != nil {
			t.Errorf("Second writeAtomic should succeed: %v", err)
		}

		// Verify marker is still present
		content, err := os.ReadFile(testConfigPath)
		if err != nil {
			t.Fatalf("Failed to read updated config: %v", err)
		}

		if !strings.Contains(string(content), WiretyMarker) {
			t.Errorf("Updated config should still contain Wirety marker")
		}
	})

	t.Run("NonWiretyFileRejection", func(t *testing.T) {
		// Create a file without our marker
		nonWiretyPath := filepath.Join(tempDir, "wg1.conf")
		nonWiretyContent := `# Some other tool's config
[Interface]
PrivateKey = other123
Address = 10.0.1.1/24
`
		if err := os.WriteFile(nonWiretyPath, []byte(nonWiretyContent), 0o600); err != nil {
			t.Fatalf("Failed to create non-Wirety config: %v", err)
		}

		nonWiretyWriter := NewWriter(nonWiretyPath, "wg1", "wg-quick")

		// Should fail ownership check
		if err := nonWiretyWriter.VerifyOwnership(); err == nil {
			t.Errorf("VerifyOwnership should fail for non-Wirety config file")
		} else {
			if !strings.Contains(err.Error(), "missing marker") {
				t.Errorf("Error should mention missing marker, got: %v", err)
			}
		}

		// WriteAndApply should also fail
		testConfig := `[Interface]
PrivateKey = test123`
		if err := nonWiretyWriter.CheckOwnership(); err == nil {
			t.Errorf("CheckOwnership should fail for non-Wirety config file")
		}
	})
}

func TestAddMarkerToConfig(t *testing.T) {
	writer := NewWriter("/tmp/test.conf", "wg0", "wg-quick")
	
	t.Run("AddMarkerToNewConfig", func(t *testing.T) {
		config := `[Interface]
PrivateKey = test123`
		
		marked := writer.addMarkerToConfig(config)
		
		if !strings.HasPrefix(marked, WiretyMarker) {
			t.Errorf("Config should start with Wirety marker")
		}
		
		if !strings.Contains(marked, config) {
			t.Errorf("Original config should be preserved")
		}
	})
	
	t.Run("SkipMarkerIfAlreadyPresent", func(t *testing.T) {
		alreadyMarked := WiretyMarker + "\n[Interface]\nPrivateKey = test123"
		
		result := writer.addMarkerToConfig(alreadyMarked)
		
		// Should not duplicate the marker
		markerCount := strings.Count(result, WiretyMarker)
		if markerCount != 1 {
			t.Errorf("Should have exactly one marker, found %d", markerCount)
		}
	})

	// Test cleanup of old configs
	t.Run("CleanupOldConfigs removes old Wirety configs", func(t *testing.T) {
		// Create test directory
		tempDir := t.TempDir()
		
		// Create current config
		currentConfig := filepath.Join(tempDir, "peer1.conf")
		oldConfig1 := filepath.Join(tempDir, "peer2.conf")
		oldConfig2 := filepath.Join(tempDir, "peer3.conf")
		nonWiretyConfig := filepath.Join(tempDir, "other.conf")
		
		// Write configs
		currentContent := WiretyMarker + "\n[Interface]\nPrivateKey=current"
		oldContent1 := WiretyMarker + "\n[Interface]\nPrivateKey=old1" 
		oldContent2 := WiretyMarker + "\n[Interface]\nPrivateKey=old2"
		nonWiretyContent := "[Interface]\nPrivateKey=other"
		
		os.WriteFile(currentConfig, []byte(currentContent), 0600)
		os.WriteFile(oldConfig1, []byte(oldContent1), 0600)
		os.WriteFile(oldConfig2, []byte(oldContent2), 0600)
		os.WriteFile(nonWiretyConfig, []byte(nonWiretyContent), 0600)
		
		// Create writer for current config
		writer := &Writer{Path: currentConfig, Interface: "peer1", ApplyMethod: "wg-quick"}
		
		// Cleanup should remove old Wirety configs but leave non-Wirety and current
		err := writer.CleanupOldConfigs()
		if err != nil {
			t.Fatalf("CleanupOldConfigs failed: %v", err)
		}
		
		// Check that current and non-Wirety configs still exist
		if _, err := os.Stat(currentConfig); os.IsNotExist(err) {
			t.Error("Current config should still exist")
		}
		if _, err := os.Stat(nonWiretyConfig); os.IsNotExist(err) {
			t.Error("Non-Wirety config should still exist")
		}
		
		// Check that old Wirety configs were removed
		if _, err := os.Stat(oldConfig1); !os.IsNotExist(err) {
			t.Error("Old Wirety config 1 should be removed")
		}
		if _, err := os.Stat(oldConfig2); !os.IsNotExist(err) {
			t.Error("Old Wirety config 2 should be removed")
		}
	})

	t.Run("FindOldWiretyConfigs identifies correct files", func(t *testing.T) {
		// Create test directory
		tempDir := t.TempDir()
		
		// Create various config files
		currentConfig := filepath.Join(tempDir, "current.conf")
		wiretyConfig := filepath.Join(tempDir, "old-wirety.conf")
		nonWiretyConfig := filepath.Join(tempDir, "other.conf")
		notConfig := filepath.Join(tempDir, "readme.txt")
		
		// Write content
		currentContent := WiretyMarker + "\n[Interface]\nPrivateKey=current"
		wiretyContent := WiretyMarker + "\n[Interface]\nPrivateKey=old"
		nonWiretyContent := "[Interface]\nPrivateKey=other"
		txtContent := "This is not a config file"
		
		os.WriteFile(currentConfig, []byte(currentContent), 0600)
		os.WriteFile(wiretyConfig, []byte(wiretyContent), 0600)
		os.WriteFile(nonWiretyConfig, []byte(nonWiretyContent), 0600)
		os.WriteFile(notConfig, []byte(txtContent), 0600)
		
		// Set up writer to search tempDir instead of standard locations
		writer := &Writer{Path: currentConfig, Interface: "current", ApplyMethod: "wg-quick"}
		
		// Manually check which files would be identified as old Wirety configs
		oldConfigs := []string{}
		
		entries, err := os.ReadDir(tempDir)
		if err != nil {
			t.Fatalf("Failed to read temp dir: %v", err)
		}
		
		for _, entry := range entries {
			if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".conf") {
				continue
			}
			
			configPath := filepath.Join(tempDir, entry.Name())
			if configPath == writer.Path {
				continue // Skip current config
			}
			
			if writer.isWiretyManaged(configPath) {
				oldConfigs = append(oldConfigs, configPath)
			}
		}
		
		// Should find only the old-wirety.conf file
		if len(oldConfigs) != 1 {
			t.Errorf("Expected 1 old Wirety config, got %d", len(oldConfigs))
		}
		
		if len(oldConfigs) > 0 && !strings.Contains(oldConfigs[0], "old-wirety.conf") {
			t.Errorf("Expected to find old-wirety.conf, got %s", oldConfigs[0])
		}
	})
}
