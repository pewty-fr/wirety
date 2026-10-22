package network

import (
	"encoding/json"
	"testing"
)

// An update request tells a field left out (nil: unchanged) from a field
// sent empty ("": cleared).
func TestUpdateRequestsTellAbsentFromEmpty(t *testing.T) {
	var absent, empty PolicyUpdateRequest
	mustDecode(t, `{"name":"web"}`, &absent)
	mustDecode(t, `{"description":""}`, &empty)
	if absent.Description != nil {
		t.Errorf("absent description decoded as %q, want nil", *absent.Description)
	}
	if empty.Description == nil || *empty.Description != "" {
		t.Errorf("empty description decoded as %v, want an empty string", empty.Description)
	}

	var group GroupUpdateRequest
	mustDecode(t, `{"description":""}`, &group)
	if group.Description == nil || *group.Description != "" {
		t.Errorf("group: empty description decoded as %v", group.Description)
	}

	var route RouteUpdateRequest
	mustDecode(t, `{"destination_cidr_v6":"","description":"dc"}`, &route)
	if route.DestinationCIDRv6 == nil || *route.DestinationCIDRv6 != "" || route.DestinationCIDR != nil {
		t.Errorf("route: got v4 %v, v6 %v", route.DestinationCIDR, route.DestinationCIDRv6)
	}
	if err := route.Validate(); err != nil {
		t.Errorf("clearing a destination is a valid request: %v", err)
	}

	var dns DNSMappingUpdateRequest
	mustDecode(t, `{"ip_address_v6":""}`, &dns)
	if dns.IPv6Address == nil || *dns.IPv6Address != "" || dns.IPAddress != nil {
		t.Errorf("dns: got v4 %v, v6 %v", dns.IPAddress, dns.IPv6Address)
	}
	if err := dns.Validate(); err != nil {
		t.Errorf("clearing an address is a valid request: %v", err)
	}

	var peer PeerUpdateRequest
	mustDecode(t, `{"owner_id":"","name":"laptop"}`, &peer)
	if peer.OwnerID == nil || *peer.OwnerID != "" || peer.Endpoint != nil {
		t.Errorf("peer: got owner %v, endpoint %v", peer.OwnerID, peer.Endpoint)
	}
}

func mustDecode(t *testing.T, body string, v any) {
	t.Helper()
	if err := json.Unmarshal([]byte(body), v); err != nil {
		t.Fatal(err)
	}
}
