package memory

import (
	"context"
	"fmt"
	"sync"
	"time"

	"wirety/internal/domain/network"
)

// Repository is an in-memory implementation of the network repository
type Repository struct {
	mu               sync.RWMutex
	networks         map[string]*network.Network
	connections      map[string]map[string]*network.PeerConnection // networkID -> connectionKey -> PeerConnection
	sessions         map[string]map[string]*network.AgentSession   // networkID -> sessionID -> AgentSession
	endpointChanges  map[string][]*network.EndpointChange          // networkID -> []EndpointChange
	incidents        map[string]*network.SecurityIncident          // incidentID -> SecurityIncident
	captiveWhitelist map[string]map[string]bool                    // "networkID:jumpPeerID" -> peerIP -> true
	captiveTokens    map[string]*network.CaptivePortalToken        // token -> CaptivePortalToken
}

// NewRepository creates a new in-memory repository
func NewRepository() *Repository {
	repo := &Repository{
		networks:        make(map[string]*network.Network),
		connections:     make(map[string]map[string]*network.PeerConnection),
		sessions:        make(map[string]map[string]*network.AgentSession),
		endpointChanges: make(map[string][]*network.EndpointChange),
		incidents:       make(map[string]*network.SecurityIncident),
	}
	return repo
}

// (IPAM operations removed - now handled by dedicated IPAM repository)

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
	net.PeerCount = len(net.Peers)

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
		net.PeerCount = len(net.Peers)
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

// GetPeerByToken finds a peer by its enrollment token
func (r *Repository) GetPeerByToken(ctx context.Context, token string) (string, *network.Peer, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	for networkID, net := range r.networks {
		for _, peer := range net.Peers {
			if peer.Token == token {
				return networkID, peer, nil
			}
		}
	}

	return "", nil, fmt.Errorf("token not found")
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

// Agent session operations

// CreateOrUpdateSession creates or updates an agent session
func (r *Repository) CreateOrUpdateSession(ctx context.Context, networkID string, session *network.AgentSession) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.sessions[networkID] == nil {
		r.sessions[networkID] = make(map[string]*network.AgentSession)
	}

	r.sessions[networkID][session.SessionID] = session
	return nil
}

// GetSession retrieves a specific session by session ID
func (r *Repository) GetSession(ctx context.Context, networkID, sessionID string) (*network.AgentSession, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	if r.sessions[networkID] == nil {
		return nil, fmt.Errorf("session not found")
	}

	session, exists := r.sessions[networkID][sessionID]
	if !exists {
		return nil, fmt.Errorf("session not found")
	}

	return session, nil
}

// GetActiveSessionsForPeer retrieves all active sessions for a specific peer
func (r *Repository) GetActiveSessionsForPeer(ctx context.Context, networkID, peerID string) ([]*network.AgentSession, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var sessions []*network.AgentSession
	if r.sessions[networkID] == nil {
		return sessions, nil
	}

	for _, session := range r.sessions[networkID] {
		if session.PeerID == peerID {
			sessions = append(sessions, session)
		}
	}

	return sessions, nil
}

// DeleteSession deletes a session
func (r *Repository) DeleteSession(ctx context.Context, networkID, sessionID string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.sessions[networkID] == nil {
		return fmt.Errorf("session not found")
	}

	delete(r.sessions[networkID], sessionID)
	return nil
}

// ListSessions lists all sessions in a network
func (r *Repository) ListSessions(ctx context.Context, networkID string) ([]*network.AgentSession, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var sessions []*network.AgentSession
	if r.sessions[networkID] == nil {
		return sessions, nil
	}

	for _, session := range r.sessions[networkID] {
		sessions = append(sessions, session)
	}

	return sessions, nil
}

// Endpoint change tracking

// RecordEndpointChange records an endpoint change for a peer
func (r *Repository) RecordEndpointChange(ctx context.Context, networkID string, change *network.EndpointChange) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.endpointChanges[networkID] == nil {
		r.endpointChanges[networkID] = make([]*network.EndpointChange, 0)
	}

	r.endpointChanges[networkID] = append(r.endpointChanges[networkID], change)
	return nil
}

// GetEndpointChanges retrieves endpoint changes for a peer since a given time
func (r *Repository) GetEndpointChanges(ctx context.Context, networkID, peerID string, since time.Time) ([]*network.EndpointChange, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var changes []*network.EndpointChange
	if r.endpointChanges[networkID] == nil {
		return changes, nil
	}

	for _, change := range r.endpointChanges[networkID] {
		if change.PeerID == peerID && change.ChangedAt.After(since) {
			changes = append(changes, change)
		}
	}

	return changes, nil
}

func (r *Repository) DeleteEndpointChanges(ctx context.Context, networkID, peerID string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.endpointChanges[networkID] == nil {
		return nil
	}

	// Filter out changes for this peer
	filtered := make([]*network.EndpointChange, 0)
	for _, change := range r.endpointChanges[networkID] {
		if change.PeerID != peerID {
			filtered = append(filtered, change)
		}
	}

	r.endpointChanges[networkID] = filtered
	return nil
}

// Security incident operations

// CreateSecurityIncident creates a new security incident
func (r *Repository) CreateSecurityIncident(ctx context.Context, incident *network.SecurityIncident) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.incidents[incident.ID] = incident
	return nil
}

// GetSecurityIncident retrieves a security incident by ID
func (r *Repository) GetSecurityIncident(ctx context.Context, incidentID string) (*network.SecurityIncident, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	incident, exists := r.incidents[incidentID]
	if !exists {
		return nil, fmt.Errorf("incident not found")
	}

	return incident, nil
}

// ListSecurityIncidents lists all security incidents, optionally filtered by resolved status
func (r *Repository) ListSecurityIncidents(ctx context.Context, resolved *bool) ([]*network.SecurityIncident, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var incidents []*network.SecurityIncident
	for _, incident := range r.incidents {
		if resolved == nil || incident.Resolved == *resolved {
			incidents = append(incidents, incident)
		}
	}

	return incidents, nil
}

// ListSecurityIncidentsByNetwork lists security incidents for a specific network
func (r *Repository) ListSecurityIncidentsByNetwork(ctx context.Context, networkID string, resolved *bool) ([]*network.SecurityIncident, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var incidents []*network.SecurityIncident
	for _, incident := range r.incidents {
		if incident.NetworkID == networkID {
			if resolved == nil || incident.Resolved == *resolved {
				incidents = append(incidents, incident)
			}
		}
	}

	return incidents, nil
}

// ResolveSecurityIncident marks a security incident as resolved
func (r *Repository) ResolveSecurityIncident(ctx context.Context, incidentID, resolvedBy string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	incident, exists := r.incidents[incidentID]
	if !exists {
		return fmt.Errorf("incident not found")
	}

	incident.Resolved = true
	incident.ResolvedAt = time.Now()
	incident.ResolvedBy = resolvedBy

	return nil
}

// Captive portal whitelist operations

func (r *Repository) AddCaptivePortalWhitelist(ctx context.Context, networkID, jumpPeerID, peerIP string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	key := networkID + ":" + jumpPeerID
	if r.captiveWhitelist == nil {
		r.captiveWhitelist = make(map[string]map[string]bool)
	}
	if r.captiveWhitelist[key] == nil {
		r.captiveWhitelist[key] = make(map[string]bool)
	}
	r.captiveWhitelist[key][peerIP] = true
	return nil
}

func (r *Repository) RemoveCaptivePortalWhitelist(ctx context.Context, networkID, jumpPeerID, peerIP string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	key := networkID + ":" + jumpPeerID
	if r.captiveWhitelist != nil && r.captiveWhitelist[key] != nil {
		delete(r.captiveWhitelist[key], peerIP)
	}
	return nil
}

func (r *Repository) GetCaptivePortalWhitelist(ctx context.Context, networkID, jumpPeerID string) ([]string, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	key := networkID + ":" + jumpPeerID
	var ips []string
	if r.captiveWhitelist != nil && r.captiveWhitelist[key] != nil {
		for ip := range r.captiveWhitelist[key] {
			ips = append(ips, ip)
		}
	}
	return ips, nil
}

func (r *Repository) ClearCaptivePortalWhitelist(ctx context.Context, networkID, jumpPeerID string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	key := networkID + ":" + jumpPeerID
	if r.captiveWhitelist != nil {
		delete(r.captiveWhitelist, key)
	}
	return nil
}

// Captive portal token operations

func (r *Repository) CreateCaptivePortalToken(ctx context.Context, token *network.CaptivePortalToken) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.captiveTokens == nil {
		r.captiveTokens = make(map[string]*network.CaptivePortalToken)
	}

	r.captiveTokens[token.Token] = token
	return nil
}

func (r *Repository) GetCaptivePortalToken(ctx context.Context, tokenStr string) (*network.CaptivePortalToken, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	if r.captiveTokens == nil {
		return nil, fmt.Errorf("token not found")
	}

	token, exists := r.captiveTokens[tokenStr]
	if !exists {
		return nil, fmt.Errorf("token not found")
	}

	return token, nil
}

func (r *Repository) DeleteCaptivePortalToken(ctx context.Context, tokenStr string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.captiveTokens != nil {
		delete(r.captiveTokens, tokenStr)
	}

	return nil
}

func (r *Repository) CleanupExpiredCaptivePortalTokens(ctx context.Context) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.captiveTokens == nil {
		return nil
	}

	now := time.Now()
	for token, t := range r.captiveTokens {
		if now.After(t.ExpiresAt) {
			delete(r.captiveTokens, token)
		}
	}

	return nil
}
