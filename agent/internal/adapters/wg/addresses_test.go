package wg

import (
	"net/netip"
	"reflect"
	"testing"
)

func prefixes(t *testing.T, ss ...string) []netip.Prefix {
	t.Helper()
	var out []netip.Prefix
	for _, s := range ss {
		out = append(out, netip.MustParsePrefix(s))
	}
	return out
}

func TestConfigAddresses(t *testing.T) {
	cfg := `# This file is managed by Wirety Agent - DO NOT EDIT MANUALLY
[Interface]
# Name: jump-1
PrivateKey = aaaa
Address = 10.90.0.1/32, fd29:8fc:685a:916e::1
ListenPort = 51820

[Peer]
PublicKey = bbbb
AllowedIPs = 10.90.0.2/32, fd29:8fc:685a:916e::2/128
`
	got, err := configAddresses(cfg)
	if err != nil {
		t.Fatal(err)
	}
	// A bare address is a host prefix; AllowedIPs are not addresses.
	want := prefixes(t, "10.90.0.1/32", "fd29:8fc:685a:916e::1/128")
	if !reflect.DeepEqual(got, want) {
		t.Errorf("configAddresses = %v, want %v", got, want)
	}
}

func TestInterfaceAddresses(t *testing.T) {
	out := `7: jump-1    inet 10.90.0.1/32 scope global jump-1\       valid_lft forever preferred_lft forever
7: jump-1    inet6 fd29:8fc:685a:916e::1/128 scope global \       valid_lft forever preferred_lft forever
`
	got, err := interfaceAddresses(out)
	if err != nil {
		t.Fatal(err)
	}
	want := prefixes(t, "10.90.0.1/32", "fd29:8fc:685a:916e::1/128")
	if !reflect.DeepEqual(got, want) {
		t.Errorf("interfaceAddresses = %v, want %v", got, want)
	}
}

func TestAddressDiff(t *testing.T) {
	tests := []struct {
		name          string
		want, have    []string
		wantAdd, wDel []string
	}{
		{
			// The reported bug: interface created IPv4-only, IPv6 added later.
			name:    "IPv6 added to an existing interface",
			want:    []string{"10.90.0.1/32", "fd29:8fc:685a:916e::1/128"},
			have:    []string{"10.90.0.1/32"},
			wantAdd: []string{"fd29:8fc:685a:916e::1/128"},
		},
		{
			name:    "address changed",
			want:    []string{"10.90.0.5/32"},
			have:    []string{"10.90.0.1/32"},
			wantAdd: []string{"10.90.0.5/32"},
			wDel:    []string{"10.90.0.1/32"},
		},
		{
			name: "in sync",
			want: []string{"10.90.0.1/32", "fd00::1/128"},
			have: []string{"fd00::1/128", "10.90.0.1/32"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			add, del := addressDiff(prefixes(t, tt.want...), prefixes(t, tt.have...))
			if !reflect.DeepEqual(add, prefixes(t, tt.wantAdd...)) {
				t.Errorf("add = %v, want %v", add, tt.wantAdd)
			}
			if !reflect.DeepEqual(del, prefixes(t, tt.wDel...)) {
				t.Errorf("del = %v, want %v", del, tt.wDel)
			}
		})
	}
}
