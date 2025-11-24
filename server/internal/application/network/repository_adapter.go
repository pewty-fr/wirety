package network

import (
	"context"
	"time"

	"wirety/internal/domain/ipam"
	"wirety/internal/domain/network"
)

// FullRepository composes network and ipam repository capabilities so legacy
// service code can remain unchanged while IPAM is split at the domain level.
type FullRepository interface {
	network.Repository
	ipam.Repository
}

// CombinedRepository implements FullRepository by delegation.
type CombinedRepository struct {
	netRepo  network.Repository
	ipamRepo ipam.Repository
}

func NewCombinedRepository(netRepo network.Repository, ipamRepo ipam.Repository) *CombinedRepository {
	return &CombinedRepository{netRepo: netRepo, ipamRepo: ipamRepo}
}

// Delegate network.Repository methods
func (c *CombinedRepository) CreateNetwork(ctx context.Context, n *network.Network) error {
	return c.netRepo.CreateNetwork(ctx, n)
}
func (c *CombinedRepository) GetNetwork(ctx context.Context, id string) (*network.Network, error) {
	return c.netRepo.GetNetwork(ctx, id)
}
func (c *CombinedRepository) UpdateNetwork(ctx context.Context, n *network.Network) error {
	return c.netRepo.UpdateNetwork(ctx, n)
}
func (c *CombinedRepository) DeleteNetwork(ctx context.Context, id string) error {
	return c.netRepo.DeleteNetwork(ctx, id)
}
func (c *CombinedRepository) ListNetworks(ctx context.Context) ([]*network.Network, error) {
	return c.netRepo.ListNetworks(ctx)
}
func (c *CombinedRepository) CreatePeer(ctx context.Context, networkID string, p *network.Peer) error {
	return c.netRepo.CreatePeer(ctx, networkID, p)
}
func (c *CombinedRepository) GetPeer(ctx context.Context, networkID, peerID string) (*network.Peer, error) {
	return c.netRepo.GetPeer(ctx, networkID, peerID)
}
func (c *CombinedRepository) GetPeerByToken(ctx context.Context, token string) (string, *network.Peer, error) {
	return c.netRepo.GetPeerByToken(ctx, token)
}
func (c *CombinedRepository) UpdatePeer(ctx context.Context, networkID string, p *network.Peer) error {
	return c.netRepo.UpdatePeer(ctx, networkID, p)
}
func (c *CombinedRepository) DeletePeer(ctx context.Context, networkID, peerID string) error {
	return c.netRepo.DeletePeer(ctx, networkID, peerID)
}
func (c *CombinedRepository) ListPeers(ctx context.Context, networkID string) ([]*network.Peer, error) {
	return c.netRepo.ListPeers(ctx, networkID)
}
func (c *CombinedRepository) CreateACL(ctx context.Context, networkID string, acl *network.ACL) error {
	return c.netRepo.CreateACL(ctx, networkID, acl)
}
func (c *CombinedRepository) GetACL(ctx context.Context, networkID string) (*network.ACL, error) {
	return c.netRepo.GetACL(ctx, networkID)
}
func (c *CombinedRepository) UpdateACL(ctx context.Context, networkID string, acl *network.ACL) error {
	return c.netRepo.UpdateACL(ctx, networkID, acl)
}
func (c *CombinedRepository) CreateConnection(ctx context.Context, networkID string, conn *network.PeerConnection) error {
	return c.netRepo.CreateConnection(ctx, networkID, conn)
}
func (c *CombinedRepository) GetConnection(ctx context.Context, networkID, p1, p2 string) (*network.PeerConnection, error) {
	return c.netRepo.GetConnection(ctx, networkID, p1, p2)
}
func (c *CombinedRepository) ListConnections(ctx context.Context, networkID string) ([]*network.PeerConnection, error) {
	return c.netRepo.ListConnections(ctx, networkID)
}
func (c *CombinedRepository) DeleteConnection(ctx context.Context, networkID, p1, p2 string) error {
	return c.netRepo.DeleteConnection(ctx, networkID, p1, p2)
}
func (c *CombinedRepository) CreateOrUpdateSession(ctx context.Context, networkID string, s *network.AgentSession) error {
	return c.netRepo.CreateOrUpdateSession(ctx, networkID, s)
}
func (c *CombinedRepository) GetSession(ctx context.Context, networkID, peerID string) (*network.AgentSession, error) {
	return c.netRepo.GetSession(ctx, networkID, peerID)
}
func (c *CombinedRepository) GetActiveSessionsForPeer(ctx context.Context, networkID, peerID string) ([]*network.AgentSession, error) {
	return c.netRepo.GetActiveSessionsForPeer(ctx, networkID, peerID)
}
func (c *CombinedRepository) DeleteSession(ctx context.Context, networkID, sessionID string) error {
	return c.netRepo.DeleteSession(ctx, networkID, sessionID)
}
func (c *CombinedRepository) ListSessions(ctx context.Context, networkID string) ([]*network.AgentSession, error) {
	return c.netRepo.ListSessions(ctx, networkID)
}
func (c *CombinedRepository) RecordEndpointChange(ctx context.Context, networkID string, change *network.EndpointChange) error {
	return c.netRepo.RecordEndpointChange(ctx, networkID, change)
}
func (c *CombinedRepository) GetEndpointChanges(ctx context.Context, networkID, peerID string, since time.Time) ([]*network.EndpointChange, error) {
	return c.netRepo.GetEndpointChanges(ctx, networkID, peerID, since)
}

func (c *CombinedRepository) DeleteEndpointChanges(ctx context.Context, networkID, peerID string) error {
	return c.netRepo.DeleteEndpointChanges(ctx, networkID, peerID)
}
func (c *CombinedRepository) CreateSecurityIncident(ctx context.Context, incident *network.SecurityIncident) error {
	return c.netRepo.CreateSecurityIncident(ctx, incident)
}
func (c *CombinedRepository) GetSecurityIncident(ctx context.Context, id string) (*network.SecurityIncident, error) {
	return c.netRepo.GetSecurityIncident(ctx, id)
}
func (c *CombinedRepository) ListSecurityIncidents(ctx context.Context, resolved *bool) ([]*network.SecurityIncident, error) {
	return c.netRepo.ListSecurityIncidents(ctx, resolved)
}
func (c *CombinedRepository) ListSecurityIncidentsByNetwork(ctx context.Context, networkID string, resolved *bool) ([]*network.SecurityIncident, error) {
	return c.netRepo.ListSecurityIncidentsByNetwork(ctx, networkID, resolved)
}
func (c *CombinedRepository) ResolveSecurityIncident(ctx context.Context, incidentID, resolvedBy string) error {
	return c.netRepo.ResolveSecurityIncident(ctx, incidentID, resolvedBy)
}

// Delegate ipam.Repository methods
func (c *CombinedRepository) EnsureRootPrefix(ctx context.Context, cidr string) (*network.IPAMPrefix, error) {
	return c.ipamRepo.EnsureRootPrefix(ctx, cidr)
}
func (c *CombinedRepository) AcquireChildPrefix(ctx context.Context, parentCIDR string, prefixLen uint8) (*network.IPAMPrefix, error) {
	return c.ipamRepo.AcquireChildPrefix(ctx, parentCIDR, prefixLen)
}
func (c *CombinedRepository) AcquireSpecificChildPrefix(ctx context.Context, parentCIDR string, cidr string) (*network.IPAMPrefix, error) {
	return c.ipamRepo.AcquireSpecificChildPrefix(ctx, parentCIDR, cidr)
}
func (c *CombinedRepository) ReleaseChildPrefix(ctx context.Context, cidr string) error {
	return c.ipamRepo.ReleaseChildPrefix(ctx, cidr)
}
func (c *CombinedRepository) DeletePrefix(ctx context.Context, cidr string) error {
	return c.ipamRepo.DeletePrefix(ctx, cidr)
}
func (c *CombinedRepository) ListChildPrefixes(ctx context.Context, parentCIDR string) ([]*network.IPAMPrefix, error) {
	return c.ipamRepo.ListChildPrefixes(ctx, parentCIDR)
}
func (c *CombinedRepository) AcquireIP(ctx context.Context, cidr string) (string, error) {
	return c.ipamRepo.AcquireIP(ctx, cidr)
}
func (c *CombinedRepository) ReleaseIP(ctx context.Context, cidr string, ip string) error {
	return c.ipamRepo.ReleaseIP(ctx, cidr, ip)
}

var _ FullRepository = (*CombinedRepository)(nil)
