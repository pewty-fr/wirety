package firewall

import (
	"reflect"
	"testing"
)

func TestParseHostPorts(t *testing.T) {
	tests := []struct {
		in   string
		want []HostPort
	}{
		{"", nil},
		{"22", []HostPort{{Proto: "tcp", Port: "22"}}},
		{"22/tcp, 9100 ,51000-51010/UDP", []HostPort{
			{Proto: "tcp", Port: "22"},
			{Proto: "tcp", Port: "9100"},
			{Proto: "udp", Port: "51000:51010"},
		}},
		{"22,,", []HostPort{{Proto: "tcp", Port: "22"}}},
	}
	for _, tt := range tests {
		got, err := ParseHostPorts(tt.in)
		if err != nil {
			t.Fatalf("ParseHostPorts(%q): %v", tt.in, err)
		}
		if !reflect.DeepEqual(got, tt.want) {
			t.Errorf("ParseHostPorts(%q) = %v, want %v", tt.in, got, tt.want)
		}
	}
}

func TestParseHostPortsRejectsInvalid(t *testing.T) {
	for _, in := range []string{"ssh", "0", "65536", "22/icmp", "22-", "30-20", "22/tcp/udp"} {
		if got, err := ParseHostPorts(in); err == nil {
			t.Errorf("ParseHostPorts(%q) = %v, want an error", in, got)
		}
	}
}
