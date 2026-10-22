package ipam

import (
	"context"
	"errors"
	"net/netip"
	"slices"
	"testing"

	"wirety/internal/domain/network"
)

func suggest(t *testing.T, base string, maxPeers, count int, inUse ...string) (int, []string, error) {
	t.Helper()
	return NewService().SuggestCIDRs(context.Background(), base, maxPeers, count, inUse)
}

func TestSuggestCIDRs_SizesRangesForPeers(t *testing.T) {
	for _, tt := range []struct {
		maxPeers   int
		wantPrefix int
	}{
		{1, 30}, {2, 30}, {3, 29}, {10, 28}, {100, 25}, {254, 24}, {255, 23}, {1000, 22}, {1000000, 12},
	} {
		prefixLen, cidrs, err := suggest(t, "10.0.0.0/8", tt.maxPeers, 1)
		if err != nil {
			t.Fatalf("maxPeers=%d: %v", tt.maxPeers, err)
		}
		if prefixLen != tt.wantPrefix || len(cidrs) != 1 {
			t.Errorf("maxPeers=%d: got /%d %v, want one /%d", tt.maxPeers, prefixLen, cidrs, tt.wantPrefix)
		}
	}
}

func TestSuggestCIDRs_ConsecutiveRangesInBase(t *testing.T) {
	_, cidrs, err := suggest(t, "10.0.0.0/8", 100, 3)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"10.0.0.0/25", "10.0.0.128/25", "10.0.1.0/25"}
	if !slices.Equal(cidrs, want) {
		t.Errorf("got %v, want %v", cidrs, want)
	}
}

func TestSuggestCIDRs_SkipsRangesInUse(t *testing.T) {
	// A network on the first /24, one inside the third, and one bigger range
	// covering the fifth to eighth; IPv6 ranges and junk are ignored.
	_, cidrs, err := suggest(t, "10.0.0.0/8", 200, 3,
		"10.0.0.0/24", "10.0.2.128/25", "10.0.4.0/22", "fd00::/64", "", "garbage")
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"10.0.1.0/24", "10.0.3.0/24", "10.0.8.0/24"}
	if !slices.Equal(cidrs, want) {
		t.Errorf("got %v, want %v", cidrs, want)
	}
}

func TestSuggestCIDRs_SkipsBiggerRangeInOneStep(t *testing.T) {
	// 10.0.0.0/9 in use: the first free /30 is 10.128.0.0, found without
	// walking the 2^21 /30s of the range in use.
	_, cidrs, err := suggest(t, "10.0.0.0/8", 2, 1, "10.0.0.0/9")
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(cidrs, []string{"10.128.0.0/30"}) {
		t.Errorf("got %v", cidrs)
	}
}

func TestSuggestCIDRs_FewerOrNoneWhenBaseRunsOut(t *testing.T) {
	_, cidrs, err := suggest(t, "192.168.0.0/23", 200, 5, "192.168.0.0/24")
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(cidrs, []string{"192.168.1.0/24"}) {
		t.Errorf("got %v", cidrs)
	}

	_, _, err = suggest(t, "192.168.0.0/23", 200, 5, "192.168.0.0/23")
	if !errors.Is(err, ErrNoFreeRange) {
		t.Errorf("err = %v, want ErrNoFreeRange", err)
	}
}

func TestSuggestCIDRs_SuggestionsNeverOverlap(t *testing.T) {
	_, cidrs, err := suggest(t, "10.0.0.0/8", 50, 20, "10.0.0.64/26", "10.0.1.0/24")
	if err != nil {
		t.Fatal(err)
	}
	if len(cidrs) != 20 {
		t.Fatalf("got %d suggestions", len(cidrs))
	}
	used := []netip.Prefix{netip.MustParsePrefix("10.0.0.64/26"), netip.MustParsePrefix("10.0.1.0/24")}
	for i, c := range cidrs {
		p := netip.MustParsePrefix(c)
		for _, u := range used {
			if p.Overlaps(u) {
				t.Errorf("%s overlaps %s in use", c, u)
			}
		}
		for _, other := range cidrs[i+1:] {
			if p.Overlaps(netip.MustParsePrefix(other)) {
				t.Errorf("%s overlaps %s", c, other)
			}
		}
	}
}

func TestSuggestCIDRs_RejectsBadInput(t *testing.T) {
	for _, tt := range []struct {
		name     string
		base     string
		maxPeers int
		invalid  bool // a validation error (400), not just an error
	}{
		{"zero peers", "10.0.0.0/8", 0, false},
		{"negative peers", "10.0.0.0/8", -1, false},
		{"not a CIDR", "10.0.0.0", 10, true},
		{"host bits", "10.0.0.1/8", 10, true},
		{"IPv6 base", "fd00::/48", 10, true},
		{"base too small", "10.0.0.0/24", 1000, true},
	} {
		_, _, err := suggest(t, tt.base, tt.maxPeers, 1)
		if err == nil {
			t.Errorf("%s: no error", tt.name)
			continue
		}
		if tt.invalid && !errors.Is(err, network.ErrInvalidCIDR) {
			t.Errorf("%s: err = %v, want ErrInvalidCIDR", tt.name, err)
		}
	}
}

func TestSuggestCIDRs_CountDefaultsToOne(t *testing.T) {
	for _, count := range []int{0, -1} {
		_, cidrs, err := suggest(t, "10.0.0.0/8", 10, count)
		if err != nil || len(cidrs) != 1 {
			t.Errorf("count=%d: got %v, %v", count, cidrs, err)
		}
	}
}
