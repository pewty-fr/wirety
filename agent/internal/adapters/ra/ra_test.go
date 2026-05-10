package ra

import (
	"bytes"
	"testing"
)

// TestEncodeCaptivePortalOption verifies the RFC 8910 option layout:
//
//	type (1) | length-in-8-byte-units (1) | URI (variable, NUL-padded to 8-byte boundary)
//
// Length includes the type and length bytes themselves.
func TestEncodeCaptivePortalOption(t *testing.T) {
	tests := []struct {
		name        string
		uri         string
		wantLen     int  // total option length in bytes
		wantLenByte byte // length field value (in 8-byte units)
	}{
		// "https://example.com/captive-portal"  = 34 bytes
		// + 2-byte header = 36
		// → padded to next 8-byte boundary = 40 bytes = length field 5
		{
			name:        "typical https URL",
			uri:         "https://example.com/captive-portal",
			wantLen:     40,
			wantLenByte: 5,
		},
		// 6-byte URI + 2 header = 8 → length 1, no padding
		{
			name:        "exactly aligned",
			uri:         "abcdef",
			wantLen:     8,
			wantLenByte: 1,
		},
		// 7-byte URI + 2 header = 9 → padded to 16 → length 2
		{
			name:        "needs padding",
			uri:         "abcdefg",
			wantLen:     16,
			wantLenByte: 2,
		},
		// Empty URI: 0 + 2 = 2 → padded to 8 → length 1
		{
			name:        "empty URI",
			uri:         "",
			wantLen:     8,
			wantLenByte: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			b := encodeCaptivePortalOption(tt.uri)

			if len(b) != tt.wantLen {
				t.Errorf("len = %d, want %d", len(b), tt.wantLen)
			}
			if b[0] != captivePortalOptionType {
				t.Errorf("type byte = %d, want %d", b[0], captivePortalOptionType)
			}
			if b[1] != tt.wantLenByte {
				t.Errorf("length byte = %d, want %d", b[1], tt.wantLenByte)
			}
			if !bytes.Equal(b[2:2+len(tt.uri)], []byte(tt.uri)) {
				t.Errorf("URI bytes = %q, want %q", string(b[2:2+len(tt.uri)]), tt.uri)
			}
			// Padding bytes must be NUL.
			for i := 2 + len(tt.uri); i < len(b); i++ {
				if b[i] != 0 {
					t.Errorf("expected NUL padding at offset %d, got %#x", i, b[i])
				}
			}
			// Total length must be multiple of 8.
			if len(b)%8 != 0 {
				t.Errorf("total length %d not multiple of 8", len(b))
			}
			// Length-byte * 8 must equal total length.
			if int(b[1])*8 != len(b) {
				t.Errorf("length byte (%d) * 8 = %d, doesn't match total length %d", b[1], int(b[1])*8, len(b))
			}
		})
	}
}

// TestBuildRA verifies the full RA payload structure: 12 bytes of zeroed
// RA-specific fields, followed by the captive-portal option.
func TestBuildRA(t *testing.T) {
	uri := "https://example.com/captive-portal"
	b := buildRA(uri)

	// First 12 bytes are the RA fields (all zero).
	if len(b) < 12 {
		t.Fatalf("RA body too short: %d bytes", len(b))
	}
	for i := 0; i < 12; i++ {
		if b[i] != 0 {
			t.Errorf("RA field at offset %d = %#x, want 0", i, b[i])
		}
	}

	// Bytes 12... should be the captive-portal option.
	opt := b[12:]
	if opt[0] != captivePortalOptionType {
		t.Errorf("option type = %d, want %d", opt[0], captivePortalOptionType)
	}
}
