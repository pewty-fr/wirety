package validation

import (
	"testing"
)

func TestValidateDNSName(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr bool
		errType error
	}{
		// Valid names
		{"valid simple name", "myhost", false, nil},
		{"valid with numbers", "host123", false, nil},
		{"valid with hyphens", "my-host-name", false, nil},
		{"valid mixed", "web-server-01", false, nil},
		{"single character", "a", false, nil},
		{"starts with number", "1host", false, nil},
		{"ends with number", "host1", false, nil},

		// Invalid names
		{"empty name", "", true, ErrNameEmpty},
		{"starts with hyphen", "-hostname", true, ErrNameStartsWithHyphen},
		{"ends with hyphen", "hostname-", true, ErrNameEndsWithHyphen},
		{"uppercase letters", "MyHost", true, ErrInvalidDNSName},
		{"contains spaces", "my host", true, ErrInvalidDNSName},
		{"contains underscore", "my_host", true, ErrInvalidDNSName},
		{"contains dot", "my.host", true, ErrInvalidDNSName},
		{"contains special chars", "my@host", true, ErrInvalidDNSName},
		{"too long", "verylonghostnamethatexceedsthemaximumlengthof63charactersallowed", true, ErrNameTooLong},
		{"only hyphens", "---", true, ErrNameStartsWithHyphen},
		{"consecutive hyphens", "my--host", false, nil}, // This is actually valid in DNS
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateDNSName(tt.input)

			if tt.wantErr {
				if err == nil {
					t.Errorf("ValidateDNSName() expected error but got none")
					return
				}
				if tt.errType != nil && err != tt.errType {
					t.Errorf("ValidateDNSName() error = %v, want %v", err, tt.errType)
				}
			} else {
				if err != nil {
					t.Errorf("ValidateDNSName() unexpected error = %v", err)
				}
			}
		})
	}
}

func TestSanitizeDNSName(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{"lowercase simple", "MyHost", "myhost"},
		{"replace spaces", "My Host", "my-host"},
		{"replace underscores", "my_host_name", "my-host-name"},
		{"replace special chars", "my@host#name", "my-host-name"},
		{"trim hyphens", "-my-host-", "my-host"},
		{"collapse hyphens", "my---host", "my-host"},
		{"complex sanitization", "My_Complex@Host#Name!", "my-complex-host-name"},
		{"empty string", "", ""},
		{"only special chars", "@#$%", ""},
		{"truncate long name", "verylonghostnamethatexceedsthemaximumlengthof63charactersallowedindnsnames", "verylonghostnamethatexceedsthemaximumlengthof63charactersallo"},
		{"truncate with trailing hyphen", "verylonghostnamethatexceedsthemaximumlengthof63characters-", "verylonghostnamethatexceedsthemaximumlengthof63characters"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := SanitizeDNSName(tt.input)
			if result != tt.expected {
				t.Errorf("SanitizeDNSName(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestValidateDNSNameCaseSensitivity(t *testing.T) {
	// Test that validation handles case conversion properly
	testCases := []string{"MyHost", "MYHOST", "myHOST", "MyHoSt"}

	for _, tc := range testCases {
		t.Run(tc, func(t *testing.T) {
			err := ValidateDNSName(tc)
			if err != ErrInvalidDNSName {
				t.Errorf("ValidateDNSName(%q) should reject uppercase letters, got %v", tc, err)
			}
		})
	}
}
