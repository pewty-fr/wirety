package wireguard

import (
	"net"
	"strings"
	"testing"

	domain "wirety/internal/domain/network"
)

func TestGenerateConfig(t *testing.T) {
	tests := []struct {
		name          string
		peer          *domain.Peer
		allowedPeers  []*domain.Peer
		network       *domain.Network
		presharedKeys map[string]string
		routes        []*domain.Route
		expectedParts []string
		notExpected   []string
	}{
		{
			name: "regular peer with jump server",
			peer: &domain.Peer{
				ID:         "peer1",
				Name:       "client-peer",
				PrivateKey: "private-key-1",
				Address:    "10.0.0.10",
				IsJump:     false,
			},
			allowedPeers: []*domain.Peer{
				{
					ID:         "jump1",
					Name:       "jump-server",
					PublicKey:  "public-key-jump",
					Address:    "10.0.0.1",
					IsJump:     true,
					Endpoint:   "jump.example.com",
					ListenPort: 51820,
				},
			},
			network: &domain.Network{
				CIDR: "10.0.0.0/16",
			},
			presharedKeys: map[string]string{
				"jump1": "preshared-key-123",
			},
			routes: []*domain.Route{
				{
					ID:              "route1",
					DestinationCIDR: "192.168.1.0/24",
					JumpPeerID:      "jump1",
				},
			},
			expectedParts: []string{
				"[Interface]",
				"# Name: client-peer",
				"PrivateKey = private-key-1",
				"Address = 10.0.0.10",
				"DNS = 10.0.0.1",
				"[Peer]",
				"# Name: jump-server",
				"PublicKey = public-key-jump",
				"PresharedKey = preshared-key-123",
				"AllowedIPs = 10.0.0.0/16, 192.168.1.0/24",
				"Endpoint = jump.example.com:51820",
				"PersistentKeepalive = 25",
			},
		},
		{
			name: "jump server peer",
			peer: &domain.Peer{
				ID:         "jump1",
				Name:       "jump-server",
				PrivateKey: "private-key-jump",
				Address:    "10.0.0.1",
				IsJump:     true,
				ListenPort: 51820,
			},
			allowedPeers: []*domain.Peer{
				{
					ID:        "peer1",
					Name:      "client-peer",
					PublicKey: "public-key-1",
					Address:   "10.0.0.10",
					IsJump:    false,
				},
			},
			network: &domain.Network{
				CIDR: "10.0.0.0/16",
			},
			presharedKeys: map[string]string{},
			routes: []*domain.Route{
				{
					ID:              "route1",
					DestinationCIDR: "192.168.1.0/24",
					JumpPeerID:      "jump1",
				},
			},
			expectedParts: []string{
				"[Interface]",
				"# Name: jump-server",
				"PrivateKey = private-key-jump",
				"Address = 10.0.0.1",
				"ListenPort = 51820",
				"DNS = 10.0.0.1",
				"[Peer]",
				"# Name: client-peer",
				"PublicKey = public-key-1",
				"AllowedIPs = 10.0.0.0/16, 192.168.1.0/24",
				"PersistentKeepalive = 25",
			},
			notExpected: []string{
				"PresharedKey",
				"Endpoint",
			},
		},
		{
			name: "peer with additional allowed IPs",
			peer: &domain.Peer{
				ID:                   "peer1",
				Name:                 "client-peer",
				PrivateKey:           "private-key-1",
				Address:              "10.0.0.10",
				IsJump:               false,
				AdditionalAllowedIPs: []string{"172.16.0.0/16"},
			},
			allowedPeers: []*domain.Peer{
				{
					ID:                   "jump1",
					Name:                 "jump-server",
					PublicKey:            "public-key-jump",
					Address:              "10.0.0.1",
					IsJump:               true,
					Endpoint:             "jump.example.com",
					ListenPort:           51820,
					AdditionalAllowedIPs: []string{"203.0.113.0/24"},
				},
			},
			network: &domain.Network{
				CIDR: "10.0.0.0/16",
			},
			presharedKeys: map[string]string{},
			routes:        []*domain.Route{},
			expectedParts: []string{
				"AllowedIPs = 10.0.0.0/16, 203.0.113.0/24",
			},
		},
		{
			name: "regular peer to regular peer",
			peer: &domain.Peer{
				ID:         "peer1",
				Name:       "client-peer-1",
				PrivateKey: "private-key-1",
				Address:    "10.0.0.10",
				IsJump:     false,
			},
			allowedPeers: []*domain.Peer{
				{
					ID:        "peer2",
					Name:      "client-peer-2",
					PublicKey: "public-key-2",
					Address:   "10.0.0.11",
					IsJump:    false,
				},
			},
			network: &domain.Network{
				CIDR: "10.0.0.0/16",
			},
			presharedKeys: map[string]string{},
			routes:        []*domain.Route{},
			expectedParts: []string{
				"AllowedIPs = 10.0.0.11/32",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := GenerateConfig(tt.peer, tt.allowedPeers, tt.network, tt.presharedKeys, tt.routes)

			// Check that all expected parts are present
			for _, expected := range tt.expectedParts {
				if !strings.Contains(config, expected) {
					t.Errorf("Expected config to contain %q, but it didn't. Config:\n%s", expected, config)
				}
			}

			// Check that unwanted parts are not present
			for _, notExpected := range tt.notExpected {
				if strings.Contains(config, notExpected) {
					t.Errorf("Expected config to NOT contain %q, but it did. Config:\n%s", notExpected, config)
				}
			}
		})
	}
}

func TestDetermineAllowedIPs(t *testing.T) {
	network := &domain.Network{
		CIDR: "10.0.0.0/16",
	}

	tests := []struct {
		name        string
		peer        *domain.Peer
		allowedPeer *domain.Peer
		routes      []*domain.Route
		expected    []string
	}{
		{
			name: "jump peer with routes",
			peer: &domain.Peer{
				ID:     "jump1",
				IsJump: true,
			},
			allowedPeer: &domain.Peer{
				ID:     "peer1",
				IsJump: false,
			},
			routes: []*domain.Route{
				{
					ID:              "route1",
					DestinationCIDR: "192.168.1.0/24",
				},
				{
					ID:              "route2",
					DestinationCIDR: "192.168.2.0/24",
				},
			},
			expected: []string{"10.0.0.0/16", "192.168.1.0/24", "192.168.2.0/24"},
		},
		{
			name: "regular peer to jump peer with matching routes",
			peer: &domain.Peer{
				ID:     "peer1",
				IsJump: false,
			},
			allowedPeer: &domain.Peer{
				ID:     "jump1",
				IsJump: true,
			},
			routes: []*domain.Route{
				{
					ID:              "route1",
					DestinationCIDR: "192.168.1.0/24",
					JumpPeerID:      "jump1",
				},
				{
					ID:              "route2",
					DestinationCIDR: "192.168.2.0/24",
					JumpPeerID:      "jump2", // Different jump peer
				},
			},
			expected: []string{"10.0.0.0/16", "192.168.1.0/24"},
		},
		{
			name: "regular peer to regular peer",
			peer: &domain.Peer{
				ID:     "peer1",
				IsJump: false,
			},
			allowedPeer: &domain.Peer{
				ID:      "peer2",
				Address: "10.0.0.11",
				IsJump:  false,
			},
			routes:   []*domain.Route{},
			expected: []string{"10.0.0.11/32"},
		},
		{
			name: "jump peer with additional allowed IPs",
			peer: &domain.Peer{
				ID:                   "jump1",
				IsJump:               true,
				AdditionalAllowedIPs: []string{"172.16.0.0/16", "203.0.113.0/24"},
			},
			allowedPeer: &domain.Peer{
				ID:     "peer1",
				IsJump: false,
			},
			routes:   []*domain.Route{},
			expected: []string{"10.0.0.0/16", "172.16.0.0/16", "203.0.113.0/24"},
		},
		{
			name: "regular peer to jump peer with additional allowed IPs",
			peer: &domain.Peer{
				ID:     "peer1",
				IsJump: false,
			},
			allowedPeer: &domain.Peer{
				ID:                   "jump1",
				IsJump:               true,
				AdditionalAllowedIPs: []string{"172.16.0.0/16"},
			},
			routes:   []*domain.Route{},
			expected: []string{"10.0.0.0/16", "172.16.0.0/16"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := determineAllowedIPs(tt.peer, tt.allowedPeer, network, tt.routes)

			if len(result) != len(tt.expected) {
				t.Errorf("Expected %d allowed IPs, got %d. Expected: %v, Got: %v",
					len(tt.expected), len(result), tt.expected, result)
				return
			}

			// Convert to maps for easier comparison
			expectedMap := make(map[string]bool)
			for _, ip := range tt.expected {
				expectedMap[ip] = true
			}

			resultMap := make(map[string]bool)
			for _, ip := range result {
				resultMap[ip] = true
			}

			for _, expected := range tt.expected {
				if !resultMap[expected] {
					t.Errorf("Expected allowed IP %q not found in result: %v", expected, result)
				}
			}

			for _, got := range result {
				if !expectedMap[got] {
					t.Errorf("Unexpected allowed IP %q found in result: %v", got, result)
				}
			}
		})
	}
}

func TestAllocateIP(t *testing.T) {
	tests := []struct {
		name        string
		cidr        string
		usedIPs     []string
		expectError bool
		expectIP    string
	}{
		{
			name:     "allocate first available IP",
			cidr:     "10.0.0.0/30", // Only 4 IPs: .0, .1, .2, .3
			usedIPs:  []string{},
			expectIP: "10.0.0.1/32", // .0 is network, .3 is broadcast, so .1 is first available
		},
		{
			name:     "allocate with some IPs used",
			cidr:     "10.0.0.0/30",
			usedIPs:  []string{"10.0.0.1"},
			expectIP: "10.0.0.2/32",
		},
		{
			name:     "allocate with /32 suffix in used IPs",
			cidr:     "10.0.0.0/30",
			usedIPs:  []string{"10.0.0.1/32"},
			expectIP: "10.0.0.2/32",
		},
		{
			name:        "no available IPs",
			cidr:        "10.0.0.0/30",
			usedIPs:     []string{"10.0.0.1", "10.0.0.2"},
			expectError: true,
		},
		{
			name:        "invalid CIDR",
			cidr:        "invalid-cidr",
			usedIPs:     []string{},
			expectError: true,
		},
		{
			name:     "larger subnet",
			cidr:     "192.168.1.0/28", // 16 IPs
			usedIPs:  []string{"192.168.1.1", "192.168.1.2", "192.168.1.3"},
			expectIP: "192.168.1.4/32",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := AllocateIP(tt.cidr, tt.usedIPs)

			if tt.expectError {
				if err == nil {
					t.Errorf("Expected error but got none. Result: %s", result)
				}
				return
			}

			if err != nil {
				t.Errorf("Unexpected error: %v", err)
				return
			}

			if tt.expectIP != "" && result != tt.expectIP {
				t.Errorf("Expected IP %s, got %s", tt.expectIP, result)
			}

			// Verify the result is a valid IP with /32 suffix
			if !strings.HasSuffix(result, "/32") {
				t.Errorf("Expected result to have /32 suffix, got %s", result)
			}
		})
	}
}

func TestIncIP(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "increment simple IP",
			input:    "10.0.0.1",
			expected: "10.0.0.2",
		},
		{
			name:     "increment with carry",
			input:    "10.0.0.255",
			expected: "10.0.1.0",
		},
		{
			name:     "increment with multiple carries",
			input:    "10.0.255.255",
			expected: "10.1.0.0",
		},
		{
			name:     "increment at boundary",
			input:    "192.168.1.254",
			expected: "192.168.1.255",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ip := net.ParseIP(tt.input)
			if ip == nil {
				t.Fatalf("Failed to parse input IP: %s", tt.input)
			}

			incIP(ip)

			result := ip.String()
			if result != tt.expected {
				t.Errorf("Expected %s, got %s", tt.expected, result)
			}
		})
	}
}

func TestIsNetworkOrBroadcast(t *testing.T) {
	tests := []struct {
		name     string
		ip       string
		cidr     string
		expected bool
	}{
		{
			name:     "network address",
			ip:       "10.0.0.0",
			cidr:     "10.0.0.0/24",
			expected: true,
		},
		{
			name:     "broadcast address",
			ip:       "10.0.0.255",
			cidr:     "10.0.0.0/24",
			expected: true,
		},
		{
			name:     "regular address",
			ip:       "10.0.0.1",
			cidr:     "10.0.0.0/24",
			expected: false,
		},
		{
			name:     "another regular address",
			ip:       "10.0.0.100",
			cidr:     "10.0.0.0/24",
			expected: false,
		},
		{
			name:     "network address /30",
			ip:       "192.168.1.0",
			cidr:     "192.168.1.0/30",
			expected: true,
		},
		{
			name:     "broadcast address /30",
			ip:       "192.168.1.3",
			cidr:     "192.168.1.0/30",
			expected: true,
		},
		{
			name:     "usable address /30",
			ip:       "192.168.1.1",
			cidr:     "192.168.1.0/30",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ip := net.ParseIP(tt.ip)
			if ip == nil {
				t.Fatalf("Failed to parse IP: %s", tt.ip)
			}

			_, ipnet, err := net.ParseCIDR(tt.cidr)
			if err != nil {
				t.Fatalf("Failed to parse CIDR: %s", tt.cidr)
			}

			result := isNetworkOrBroadcast(ip, ipnet)
			if result != tt.expected {
				t.Errorf("Expected %v for IP %s in CIDR %s, got %v", tt.expected, tt.ip, tt.cidr, result)
			}
		})
	}
}
