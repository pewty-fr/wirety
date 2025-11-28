package main

import (
	"testing"
)

func TestSanitizeInterfaceName(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"simple-peer", "simple-peer"},
		{"Peer_123", "peer_123"},
		{"peer@home", "peer_home"},
		{"peer with spaces", "peer_with_space"},       // truncated to 15 chars
		{"peer/with/slashes", "peer_with_slash"},      // truncated to 15 chars
		{"verylongpeernametotest", "verylongpeernam"}, // truncated to 15 chars
		{"", "wg0"}, // default if empty
		{"peer-name-with.dots", "peer-name-with"}, // truncated to 15 chars (trailing _ removed)
		{"123peer", "123peer"},                    // numbers are allowed
		{"特殊字符", "____"},                          // unicode chars replaced with underscores (4 chars)
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := sanitizeInterfaceName(tt.input)
			if result != tt.expected {
				t.Errorf("sanitizeInterfaceName(%q) = %q, want %q", tt.input, result, tt.expected)
			}

			// Verify result is valid length
			if len(result) > 15 {
				t.Errorf("sanitizeInterfaceName(%q) = %q is too long (%d chars)", tt.input, result, len(result))
			}

			// Verify result is not empty (should default to wg0)
			if result == "" {
				t.Errorf("sanitizeInterfaceName(%q) returned empty string", tt.input)
			}
		})
	}
}
