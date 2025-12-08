package network

import (
	"testing"
	"time"
)

func TestNetwork_AddPeer(t *testing.T) {
	network := &Network{
		ID:   "net1",
		Name: "test-network",
	}

	peer := &Peer{
		ID:   "peer1",
		Name: "test-peer",
	}

	// Test adding peer to empty network
	network.AddPeer(peer)

	if network.Peers == nil {
		t.Error("Expected Peers map to be initialized")
	}

	if len(network.Peers) != 1 {
		t.Errorf("Expected 1 peer, got %d", len(network.Peers))
	}

	if network.Peers["peer1"] != peer {
		t.Error("Expected peer to be added to network")
	}

	// Test that UpdatedAt is set
	if network.UpdatedAt.IsZero() {
		t.Error("Expected UpdatedAt to be set")
	}

	// Test adding another peer
	peer2 := &Peer{
		ID:   "peer2",
		Name: "test-peer-2",
	}

	oldUpdatedAt := network.UpdatedAt
	time.Sleep(time.Millisecond) // Ensure time difference
	network.AddPeer(peer2)

	if len(network.Peers) != 2 {
		t.Errorf("Expected 2 peers, got %d", len(network.Peers))
	}

	if !network.UpdatedAt.After(oldUpdatedAt) {
		t.Error("Expected UpdatedAt to be updated")
	}
}

func TestNetwork_RemovePeer(t *testing.T) {
	network := &Network{
		ID:   "net1",
		Name: "test-network",
		Peers: map[string]*Peer{
			"peer1": {ID: "peer1", Name: "test-peer-1"},
			"peer2": {ID: "peer2", Name: "test-peer-2"},
		},
	}

	// Test removing existing peer
	network.RemovePeer("peer1")

	if len(network.Peers) != 1 {
		t.Errorf("Expected 1 peer after removal, got %d", len(network.Peers))
	}

	if _, exists := network.Peers["peer1"]; exists {
		t.Error("Expected peer1 to be removed")
	}

	if _, exists := network.Peers["peer2"]; !exists {
		t.Error("Expected peer2 to still exist")
	}

	// Test that UpdatedAt is set
	if network.UpdatedAt.IsZero() {
		t.Error("Expected UpdatedAt to be set")
	}

	// Test removing non-existent peer (should not panic)
	network.RemovePeer("nonexistent")

	if len(network.Peers) != 1 {
		t.Errorf("Expected 1 peer after removing non-existent peer, got %d", len(network.Peers))
	}
}

func TestNetwork_GetPeer(t *testing.T) {
	peer1 := &Peer{ID: "peer1", Name: "test-peer-1"}
	peer2 := &Peer{ID: "peer2", Name: "test-peer-2"}

	network := &Network{
		ID:   "net1",
		Name: "test-network",
		Peers: map[string]*Peer{
			"peer1": peer1,
			"peer2": peer2,
		},
	}

	// Test getting existing peer
	gotPeer, exists := network.GetPeer("peer1")
	if !exists {
		t.Error("Expected peer1 to exist")
	}
	if gotPeer != peer1 {
		t.Error("Expected to get peer1")
	}

	// Test getting non-existent peer
	gotPeer, exists = network.GetPeer("nonexistent")
	if exists {
		t.Error("Expected nonexistent peer to not exist")
	}
	if gotPeer != nil {
		t.Error("Expected nil peer for non-existent peer")
	}
}

func TestNetwork_GetAllPeers(t *testing.T) {
	peer1 := &Peer{ID: "peer1", Name: "test-peer-1"}
	peer2 := &Peer{ID: "peer2", Name: "test-peer-2"}

	network := &Network{
		ID:   "net1",
		Name: "test-network",
		Peers: map[string]*Peer{
			"peer1": peer1,
			"peer2": peer2,
		},
	}

	peers := network.GetAllPeers()

	if len(peers) != 2 {
		t.Errorf("Expected 2 peers, got %d", len(peers))
	}

	// Check that both peers are present (order doesn't matter)
	peerMap := make(map[string]*Peer)
	for _, peer := range peers {
		peerMap[peer.ID] = peer
	}

	if peerMap["peer1"] != peer1 {
		t.Error("Expected peer1 to be in result")
	}
	if peerMap["peer2"] != peer2 {
		t.Error("Expected peer2 to be in result")
	}

	// Test empty network
	emptyNetwork := &Network{
		ID:    "net2",
		Name:  "empty-network",
		Peers: map[string]*Peer{},
	}

	emptyPeers := emptyNetwork.GetAllPeers()
	if len(emptyPeers) != 0 {
		t.Errorf("Expected 0 peers for empty network, got %d", len(emptyPeers))
	}
}

func TestNetwork_GetAllowedPeersFor(t *testing.T) {
	jumpPeer := &Peer{ID: "jump1", Name: "jump-server", IsJump: true}
	regularPeer1 := &Peer{ID: "peer1", Name: "regular-peer-1", IsJump: false}
	regularPeer2 := &Peer{ID: "peer2", Name: "regular-peer-2", IsJump: false}

	network := &Network{
		ID:   "net1",
		Name: "test-network",
		Peers: map[string]*Peer{
			"jump1": jumpPeer,
			"peer1": regularPeer1,
			"peer2": regularPeer2,
		},
	}

	// Test regular peer - should only get jump peers
	allowedForRegular := network.GetAllowedPeersFor("peer1")
	if len(allowedForRegular) != 1 {
		t.Errorf("Expected 1 allowed peer for regular peer, got %d", len(allowedForRegular))
	}
	if allowedForRegular[0].ID != "jump1" {
		t.Error("Expected regular peer to only connect to jump peer")
	}

	// Test jump peer - should get all other peers
	allowedForJump := network.GetAllowedPeersFor("jump1")
	if len(allowedForJump) != 2 {
		t.Errorf("Expected 2 allowed peers for jump peer, got %d", len(allowedForJump))
	}

	// Check that both regular peers are included
	peerMap := make(map[string]*Peer)
	for _, peer := range allowedForJump {
		peerMap[peer.ID] = peer
	}
	if peerMap["peer1"] == nil || peerMap["peer2"] == nil {
		t.Error("Expected jump peer to connect to all regular peers")
	}

	// Test non-existent peer
	allowedForNonExistent := network.GetAllowedPeersFor("nonexistent")
	if len(allowedForNonExistent) != 0 {
		t.Errorf("Expected 0 allowed peers for non-existent peer, got %d", len(allowedForNonExistent))
	}
}

func TestNetwork_HasJumpServer(t *testing.T) {
	// Test network with jump server
	networkWithJump := &Network{
		ID:   "net1",
		Name: "test-network",
		Peers: map[string]*Peer{
			"jump1": {ID: "jump1", Name: "jump-server", IsJump: true},
			"peer1": {ID: "peer1", Name: "regular-peer", IsJump: false},
		},
	}

	if !networkWithJump.HasJumpServer() {
		t.Error("Expected network to have jump server")
	}

	// Test network without jump server
	networkWithoutJump := &Network{
		ID:   "net2",
		Name: "test-network-2",
		Peers: map[string]*Peer{
			"peer1": {ID: "peer1", Name: "regular-peer-1", IsJump: false},
			"peer2": {ID: "peer2", Name: "regular-peer-2", IsJump: false},
		},
	}

	if networkWithoutJump.HasJumpServer() {
		t.Error("Expected network to not have jump server")
	}

	// Test empty network
	emptyNetwork := &Network{
		ID:    "net3",
		Name:  "empty-network",
		Peers: map[string]*Peer{},
	}

	if emptyNetwork.HasJumpServer() {
		t.Error("Expected empty network to not have jump server")
	}
}

func TestNetwork_GetJumpServers(t *testing.T) {
	jump1 := &Peer{ID: "jump1", Name: "jump-server-1", IsJump: true}
	jump2 := &Peer{ID: "jump2", Name: "jump-server-2", IsJump: true}
	regular := &Peer{ID: "peer1", Name: "regular-peer", IsJump: false}

	network := &Network{
		ID:   "net1",
		Name: "test-network",
		Peers: map[string]*Peer{
			"jump1": jump1,
			"jump2": jump2,
			"peer1": regular,
		},
	}

	jumpServers := network.GetJumpServers()

	if len(jumpServers) != 2 {
		t.Errorf("Expected 2 jump servers, got %d", len(jumpServers))
	}

	// Check that both jump servers are present
	jumpMap := make(map[string]*Peer)
	for _, jump := range jumpServers {
		jumpMap[jump.ID] = jump
	}

	if jumpMap["jump1"] != jump1 {
		t.Error("Expected jump1 to be in result")
	}
	if jumpMap["jump2"] != jump2 {
		t.Error("Expected jump2 to be in result")
	}

	// Ensure regular peer is not included
	if jumpMap["peer1"] != nil {
		t.Error("Expected regular peer to not be in jump servers result")
	}

	// Test network with no jump servers
	networkNoJumps := &Network{
		ID:   "net2",
		Name: "test-network-2",
		Peers: map[string]*Peer{
			"peer1": {ID: "peer1", Name: "regular-peer-1", IsJump: false},
			"peer2": {ID: "peer2", Name: "regular-peer-2", IsJump: false},
		},
	}

	noJumps := networkNoJumps.GetJumpServers()
	if len(noJumps) != 0 {
		t.Errorf("Expected 0 jump servers, got %d", len(noJumps))
	}
}

func TestNetwork_GetDomain(t *testing.T) {
	tests := []struct {
		name         string
		networkName  string
		domainSuffix string
		expected     string
	}{
		{
			name:         "default domain suffix",
			networkName:  "mynetwork",
			domainSuffix: "",
			expected:     "mynetwork.internal",
		},
		{
			name:         "custom domain suffix",
			networkName:  "mynetwork",
			domainSuffix: "company.local",
			expected:     "mynetwork.company.local",
		},
		{
			name:         "single word suffix",
			networkName:  "prod",
			domainSuffix: "local",
			expected:     "prod.local",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			network := &Network{
				Name:         tt.networkName,
				DomainSuffix: tt.domainSuffix,
			}

			result := network.GetDomain()
			if result != tt.expected {
				t.Errorf("Expected domain %s, got %s", tt.expected, result)
			}
		})
	}
}
