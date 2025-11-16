package memory

import (
	"context"
	"fmt"
	"net"
	"sync"

	"wirety/internal/domain/network"

	goipam "github.com/metal-stack/go-ipam"
	"github.com/rs/zerolog/log"
)

// Repository is an in-memory implementation of the network repository
type Repository struct {
	mu          sync.RWMutex
	networks    map[string]*network.Network
	ipam        goipam.Ipamer
	connections map[string]map[string]*network.PeerConnection // networkID -> connectionKey -> PeerConnection
}

// NewRepository creates a new in-memory repository
func NewRepository() *Repository {
	return &Repository{
		networks:    make(map[string]*network.Network),
		ipam:        goipam.New(context.Background()),
		connections: make(map[string]map[string]*network.PeerConnection),
	}
}

// IPAM persistence methods
func (r *Repository) EnsureRootPrefix(ctx context.Context, cidr string) (*network.IPAMPrefix, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	p, err := r.ipam.PrefixFrom(ctx, cidr)
	log.Error().Err(err).Msg("debug ensure root prefix")
	if err != nil { // not found, create
		p, err = r.ipam.NewPrefix(ctx, cidr)
		if err != nil {
			return nil, fmt.Errorf("failed to create root prefix: %w", err)
		}
	}
	usage := p.Usage()
	return &network.IPAMPrefix{CIDR: p.Cidr, UsableHosts: int(usage.AvailableIPs), ParentCIDR: ""}, nil
}

func (r *Repository) AcquireChildPrefix(ctx context.Context, parentCIDR string, prefixLen uint8) (*network.IPAMPrefix, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	child, err := r.ipam.AcquireChildPrefix(ctx, parentCIDR, prefixLen)
	if err != nil {
		return nil, err
	}
	usage := child.Usage()
	return &network.IPAMPrefix{CIDR: child.Cidr, ParentCIDR: parentCIDR, UsableHosts: int(usage.AvailableIPs)}, nil
}

func (r *Repository) ListChildPrefixes(ctx context.Context, parentCIDR string) ([]*network.IPAMPrefix, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	parentPrefix, err := r.ipam.PrefixFrom(ctx, parentCIDR)
	if err != nil {
		return nil, fmt.Errorf("parent prefix not found: %w", err)
	}
	_, parentNet, err := net.ParseCIDR(parentPrefix.Cidr)
	if err != nil {
		return nil, fmt.Errorf("invalid parent cidr")
	}
	all, err := r.ipam.ReadAllPrefixCidrs(ctx)
	if err != nil {
		return nil, fmt.Errorf("read all prefixes failed: %w", err)
	}
	out := make([]*network.IPAMPrefix, 0)
	for _, cidr := range all {
		if cidr == parentPrefix.Cidr {
			continue
		}
		_, childNet, err := net.ParseCIDR(cidr)
		if err != nil {
			continue
		}
		if parentNet.Contains(childNet.IP) && childNet.String() != parentNet.String() { // rough check for being within parent
			cp, err := r.ipam.PrefixFrom(ctx, cidr)
			if err != nil {
				continue
			}
			usage := cp.Usage()
			out = append(out, &network.IPAMPrefix{CIDR: cidr, ParentCIDR: parentCIDR, UsableHosts: int(usage.AvailableIPs)})
		}
	}
	return out, nil
}

func (r *Repository) AcquireIP(ctx context.Context, cidr string) (string, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	ipObj, err := r.ipam.AcquireIP(ctx, cidr)
	if err != nil {
		return "", err
	}
	return ipObj.IP.String(), nil
}

func (r *Repository) ReleaseIP(ctx context.Context, cidr string, ip string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.ipam.ReleaseIPFromPrefix(ctx, cidr, ip)
}

// CreateNetwork creates a new network
func (r *Repository) CreateNetwork(ctx context.Context, net *network.Network) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.networks[net.ID]; exists {
		return fmt.Errorf("network already exists")
	}

	r.networks[net.ID] = net
	return nil
}

// GetNetwork retrieves a network by ID
func (r *Repository) GetNetwork(ctx context.Context, networkID string) (*network.Network, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	net, exists := r.networks[networkID]
	if !exists {
		return nil, fmt.Errorf("network not found")
	}

	return net, nil
}

// UpdateNetwork updates a network
func (r *Repository) UpdateNetwork(ctx context.Context, net *network.Network) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.networks[net.ID]; !exists {
		return fmt.Errorf("network not found")
	}

	r.networks[net.ID] = net
	return nil
}

// DeleteNetwork deletes a network
func (r *Repository) DeleteNetwork(ctx context.Context, networkID string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.networks[networkID]; !exists {
		return fmt.Errorf("network not found")
	}

	delete(r.networks, networkID)
	return nil
}

// ListNetworks retrieves all networks
func (r *Repository) ListNetworks(ctx context.Context) ([]*network.Network, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	networks := make([]*network.Network, 0, len(r.networks))
	for _, net := range r.networks {
		networks = append(networks, net)
	}

	return networks, nil
}

// CreatePeer creates a new peer in a network
func (r *Repository) CreatePeer(ctx context.Context, networkID string, peer *network.Peer) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	net, exists := r.networks[networkID]
	if !exists {
		return fmt.Errorf("network not found")
	}

	if _, exists := net.Peers[peer.ID]; exists {
		return fmt.Errorf("peer already exists")
	}

	net.AddPeer(peer)
	return nil
}

// GetPeer retrieves a peer by ID
func (r *Repository) GetPeer(ctx context.Context, networkID, peerID string) (*network.Peer, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	net, exists := r.networks[networkID]
	if !exists {
		return nil, fmt.Errorf("network not found")
	}

	peer, exists := net.GetPeer(peerID)
	if !exists {
		return nil, fmt.Errorf("peer not found")
	}

	return peer, nil
}

// UpdatePeer updates a peer
func (r *Repository) UpdatePeer(ctx context.Context, networkID string, peer *network.Peer) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	net, exists := r.networks[networkID]
	if !exists {
		return fmt.Errorf("network not found")
	}

	if _, exists := net.Peers[peer.ID]; !exists {
		return fmt.Errorf("peer not found")
	}

	net.AddPeer(peer)
	return nil
}

// DeletePeer deletes a peer
func (r *Repository) DeletePeer(ctx context.Context, networkID, peerID string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	net, exists := r.networks[networkID]
	if !exists {
		return fmt.Errorf("network not found")
	}

	if _, exists := net.Peers[peerID]; !exists {
		return fmt.Errorf("peer not found")
	}

	net.RemovePeer(peerID)
	return nil
}

// ListPeers retrieves all peers in a network
func (r *Repository) ListPeers(ctx context.Context, networkID string) ([]*network.Peer, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	net, exists := r.networks[networkID]
	if !exists {
		return nil, fmt.Errorf("network not found")
	}

	return net.GetAllPeers(), nil
}

// CreateACL creates an ACL for a network
func (r *Repository) CreateACL(ctx context.Context, networkID string, acl *network.ACL) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	net, exists := r.networks[networkID]
	if !exists {
		return fmt.Errorf("network not found")
	}

	net.ACL = acl
	return nil
}

// GetACL retrieves the ACL for a network
func (r *Repository) GetACL(ctx context.Context, networkID string) (*network.ACL, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	net, exists := r.networks[networkID]
	if !exists {
		return nil, fmt.Errorf("network not found")
	}

	return net.ACL, nil
}

// UpdateACL updates the ACL for a network
func (r *Repository) UpdateACL(ctx context.Context, networkID string, acl *network.ACL) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	net, exists := r.networks[networkID]
	if !exists {
		return fmt.Errorf("network not found")
	}

	net.ACL = acl
	return nil
}

// PeerConnection operations

// connectionKey creates a normalized key for peer connection lookup (always peer1 < peer2)
func connectionKey(peer1ID, peer2ID string) string {
	if peer1ID < peer2ID {
		return peer1ID + "|" + peer2ID
	}
	return peer2ID + "|" + peer1ID
}

// CreateConnection creates a preshared key connection between two peers
func (r *Repository) CreateConnection(ctx context.Context, networkID string, conn *network.PeerConnection) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.connections[networkID] == nil {
		r.connections[networkID] = make(map[string]*network.PeerConnection)
	}

	key := connectionKey(conn.Peer1ID, conn.Peer2ID)
	r.connections[networkID][key] = conn
	return nil
}

// GetConnection retrieves a preshared key connection between two peers
func (r *Repository) GetConnection(ctx context.Context, networkID, peer1ID, peer2ID string) (*network.PeerConnection, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	if r.connections[networkID] == nil {
		return nil, fmt.Errorf("connection not found")
	}

	key := connectionKey(peer1ID, peer2ID)
	conn, exists := r.connections[networkID][key]
	if !exists {
		return nil, fmt.Errorf("connection not found")
	}

	return conn, nil
}

// ListConnections retrieves all connections in a network
func (r *Repository) ListConnections(ctx context.Context, networkID string) ([]*network.PeerConnection, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	conns := make([]*network.PeerConnection, 0)
	if r.connections[networkID] != nil {
		for _, conn := range r.connections[networkID] {
			conns = append(conns, conn)
		}
	}

	return conns, nil
}

// DeleteConnection removes a connection between two peers
func (r *Repository) DeleteConnection(ctx context.Context, networkID, peer1ID, peer2ID string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.connections[networkID] == nil {
		return fmt.Errorf("connection not found")
	}

	key := connectionKey(peer1ID, peer2ID)
	delete(r.connections[networkID], key)
	return nil
}
