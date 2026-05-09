package postgres

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"wirety/internal/domain/network"

	"github.com/lib/pq"
)

// NetworkRepository is a Postgres implementation of network.Repository
// ACL and IPAM are kept in-memory for now (non-persistent) to avoid schema changes.
type NetworkRepository struct {
	db   *sql.DB
	acls map[string]*network.ACL
}

// NewNetworkRepository constructs a new repository
func NewNetworkRepository(db *sql.DB) *NetworkRepository {
	return &NetworkRepository{db: db, acls: make(map[string]*network.ACL)}
}

// Network operations
func (r *NetworkRepository) CreateNetwork(ctx context.Context, n *network.Network) error {
	now := time.Now()
	n.CreatedAt = now
	n.UpdatedAt = now
	// Ensure DNS is never nil to avoid database constraint violation
	if n.DNS == nil {
		n.DNS = []string{}
	}
	_, err := r.db.ExecContext(ctx, `INSERT INTO networks (id,name,cidr,cidr_v6,dns,created_at,updated_at) VALUES ($1,$2,$3,$4,$5,$6,$7)`,
		n.ID, n.Name, n.CIDR, nullableString(n.CIDRv6), pq.Array(n.DNS), n.CreatedAt, n.UpdatedAt)
	if err != nil {
		return fmt.Errorf("create network: %w", err)
	}

	// ACL system removed - no longer stored

	return nil
}

func (r *NetworkRepository) GetNetwork(ctx context.Context, networkID string) (*network.Network, error) {
	var n network.Network
	var cidrV6 sql.NullString
	err := r.db.QueryRowContext(ctx, `SELECT id,name,cidr,cidr_v6,dns,created_at,updated_at,domain_suffix FROM networks WHERE id=$1`, networkID).
		Scan(&n.ID, &n.Name, &n.CIDR, &cidrV6, pq.Array(&n.DNS), &n.CreatedAt, &n.UpdatedAt, &n.DomainSuffix)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, fmt.Errorf("network not found")
		}
		return nil, fmt.Errorf("get network: %w", err)
	}
	n.CIDRv6 = cidrV6.String
	// Load peers
	n.Peers = make(map[string]*network.Peer)
	rows, err := r.db.QueryContext(ctx, `SELECT id,name,public_key,private_key,address,address_v6,endpoint,listen_port,additional_allowed_ips,token,is_jump,use_agent,owner_id,created_at,updated_at FROM peers WHERE network_id=$1`, networkID)
	if err != nil {
		return nil, fmt.Errorf("load peers: %w", err)
	}
	defer func() {
		_ = rows.Close()
	}()
	count := 0
	for rows.Next() {
		var p network.Peer
		var addrs []string
		var addrV6 sql.NullString
		err = rows.Scan(&p.ID, &p.Name, &p.PublicKey, &p.PrivateKey, &p.Address, &addrV6, &p.Endpoint, &p.ListenPort, pq.Array(&addrs), &p.Token, &p.IsJump, &p.UseAgent, &p.OwnerID, &p.CreatedAt, &p.UpdatedAt)
		if err != nil {
			return nil, fmt.Errorf("scan peer: %w", err)
		}
		p.AdditionalAllowedIPs = addrs
		p.AddressV6 = addrV6.String
		n.AddPeer(&p)
		count++
	}
	n.PeerCount = count
	// ACL system removed
	return &n, nil
}

func (r *NetworkRepository) UpdateNetwork(ctx context.Context, n *network.Network) error {
	n.UpdatedAt = time.Now()
	// Ensure DNS is never nil to avoid database constraint violation
	if n.DNS == nil {
		n.DNS = []string{}
	}
	_, err := r.db.ExecContext(ctx, `UPDATE networks SET name=$2,cidr=$3,cidr_v6=$4,dns=$5,updated_at=$6,domain_suffix=$7 WHERE id=$1`,
		n.ID, n.Name, n.CIDR, nullableString(n.CIDRv6), pq.Array(n.DNS), n.UpdatedAt, n.DomainSuffix)
	if err != nil {
		return fmt.Errorf("update network: %w", err)
	}
	return nil
}

func (r *NetworkRepository) DeleteNetwork(ctx context.Context, networkID string) error {
	res, err := r.db.ExecContext(ctx, `DELETE FROM networks WHERE id=$1`, networkID)
	if err != nil {
		return fmt.Errorf("delete network: %w", err)
	}
	rows, _ := res.RowsAffected()
	if rows == 0 {
		return fmt.Errorf("network not found")
	}
	delete(r.acls, networkID)
	return nil
}

func (r *NetworkRepository) ListNetworks(ctx context.Context) ([]*network.Network, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT n.id,n.name,n.cidr,n.cidr_v6,n.dns,n.created_at,n.updated_at,n.domain_suffix, COALESCE(p.peer_count,0) AS peer_count FROM networks n LEFT JOIN (SELECT network_id, COUNT(*) AS peer_count FROM peers GROUP BY network_id) p ON p.network_id = n.id ORDER BY n.created_at ASC`)
	if err != nil {
		return nil, fmt.Errorf("list networks: %w", err)
	}
	defer func() {
		_ = rows.Close()
	}()
	out := make([]*network.Network, 0)
	for rows.Next() {
		var n network.Network
		var cidrV6 sql.NullString
		err = rows.Scan(&n.ID, &n.Name, &n.CIDR, &cidrV6, pq.Array(&n.DNS), &n.CreatedAt, &n.UpdatedAt, &n.DomainSuffix, &n.PeerCount)
		if err != nil {
			return nil, err
		}
		n.CIDRv6 = cidrV6.String
		n.Peers = make(map[string]*network.Peer) // not loaded to keep call light
		// ACL system removed
		out = append(out, &n)
	}
	return out, rows.Err()
}

// Peer operations
func (r *NetworkRepository) CreatePeer(ctx context.Context, networkID string, p *network.Peer) error {
	now := time.Now()
	p.CreatedAt = now
	p.UpdatedAt = now
	// Ensure AdditionalAllowedIPs is never nil to avoid database constraint violation
	if p.AdditionalAllowedIPs == nil {
		p.AdditionalAllowedIPs = []string{}
	}
	_, err := r.db.ExecContext(ctx, `INSERT INTO peers (id,network_id,name,public_key,private_key,address,address_v6,endpoint,listen_port,additional_allowed_ips,token,is_jump,use_agent,owner_id,created_at,updated_at) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16)`,
		p.ID, networkID, p.Name, p.PublicKey, p.PrivateKey, p.Address, nullableString(p.AddressV6), p.Endpoint, p.ListenPort, pq.Array(p.AdditionalAllowedIPs), p.Token, p.IsJump, p.UseAgent, p.OwnerID, p.CreatedAt, p.UpdatedAt)
	if err != nil {
		return fmt.Errorf("create peer: %w", err)
	}
	return nil
}

func (r *NetworkRepository) GetPeer(ctx context.Context, networkID, peerID string) (*network.Peer, error) {
	var p network.Peer
	var addrs []string
	var addrV6 sql.NullString
	err := r.db.QueryRowContext(ctx, `SELECT id,name,public_key,private_key,address,address_v6,endpoint,listen_port,additional_allowed_ips,token,is_jump,use_agent,owner_id,created_at,updated_at FROM peers WHERE id=$1 AND network_id=$2`, peerID, networkID).
		Scan(&p.ID, &p.Name, &p.PublicKey, &p.PrivateKey, &p.Address, &addrV6, &p.Endpoint, &p.ListenPort, pq.Array(&addrs), &p.Token, &p.IsJump, &p.UseAgent, &p.OwnerID, &p.CreatedAt, &p.UpdatedAt)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, fmt.Errorf("peer not found")
		}
		return nil, fmt.Errorf("get peer: %w", err)
	}
	p.AdditionalAllowedIPs = addrs
	p.AddressV6 = addrV6.String

	// Load group IDs for this peer
	groupIDs, err := r.loadPeerGroupIDs(ctx, peerID)
	if err != nil {
		return nil, fmt.Errorf("load peer group IDs: %w", err)
	}
	p.GroupIDs = groupIDs

	return &p, nil
}

func (r *NetworkRepository) GetPeerByToken(ctx context.Context, token string) (string, *network.Peer, error) {
	var p network.Peer
	var networkID string
	var addrs []string
	var addrV6 sql.NullString
	err := r.db.QueryRowContext(ctx, `SELECT network_id,id,name,public_key,private_key,address,address_v6,endpoint,listen_port,additional_allowed_ips,token,is_jump,use_agent,owner_id,created_at,updated_at FROM peers WHERE token=$1`, token).
		Scan(&networkID, &p.ID, &p.Name, &p.PublicKey, &p.PrivateKey, &p.Address, &addrV6, &p.Endpoint, &p.ListenPort, pq.Array(&addrs), &p.Token, &p.IsJump, &p.UseAgent, &p.OwnerID, &p.CreatedAt, &p.UpdatedAt)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return "", nil, fmt.Errorf("token not found")
		}
		return "", nil, fmt.Errorf("get peer by token: %w", err)
	}
	p.AdditionalAllowedIPs = addrs
	p.AddressV6 = addrV6.String
	return networkID, &p, nil
}

func (r *NetworkRepository) UpdatePeer(ctx context.Context, networkID string, p *network.Peer) error {
	p.UpdatedAt = time.Now()
	// Ensure AdditionalAllowedIPs is never nil to avoid database constraint violation
	if p.AdditionalAllowedIPs == nil {
		p.AdditionalAllowedIPs = []string{}
	}
	res, err := r.db.ExecContext(ctx, `UPDATE peers SET name=$3,public_key=$4,private_key=$5,address=$6,address_v6=$7,endpoint=$8,listen_port=$9,additional_allowed_ips=$10,token=$11,is_jump=$12,use_agent=$13,owner_id=$14,updated_at=$15 WHERE id=$1 AND network_id=$2`,
		p.ID, networkID, p.Name, p.PublicKey, p.PrivateKey, p.Address, nullableString(p.AddressV6), p.Endpoint, p.ListenPort, pq.Array(p.AdditionalAllowedIPs), p.Token, p.IsJump, p.UseAgent, p.OwnerID, p.UpdatedAt)
	if err != nil {
		return fmt.Errorf("update peer: %w", err)
	}
	rows, _ := res.RowsAffected()
	if rows == 0 {
		return fmt.Errorf("peer not found")
	}
	return nil
}

func (r *NetworkRepository) DeletePeer(ctx context.Context, networkID, peerID string) error {
	res, err := r.db.ExecContext(ctx, `DELETE FROM peers WHERE id=$1 AND network_id=$2`, peerID, networkID)
	if err != nil {
		return fmt.Errorf("delete peer: %w", err)
	}
	rows, _ := res.RowsAffected()
	if rows == 0 {
		return fmt.Errorf("peer not found")
	}
	return nil
}

func (r *NetworkRepository) ListPeers(ctx context.Context, networkID string) ([]*network.Peer, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT id,name,public_key,private_key,address,address_v6,endpoint,listen_port,additional_allowed_ips,token,is_jump,use_agent,owner_id,created_at,updated_at FROM peers WHERE network_id=$1 ORDER BY created_at ASC`, networkID)
	if err != nil {
		return nil, fmt.Errorf("list peers: %w", err)
	}
	defer func() {
		_ = rows.Close()
	}()
	out := make([]*network.Peer, 0)
	for rows.Next() {
		var p network.Peer
		var addrs []string
		var addrV6 sql.NullString
		err = rows.Scan(&p.ID, &p.Name, &p.PublicKey, &p.PrivateKey, &p.Address, &addrV6, &p.Endpoint, &p.ListenPort, pq.Array(&addrs), &p.Token, &p.IsJump, &p.UseAgent, &p.OwnerID, &p.CreatedAt, &p.UpdatedAt)
		if err != nil {
			return nil, err
		}
		p.AdditionalAllowedIPs = addrs
		p.AddressV6 = addrV6.String

		// Load group IDs for this peer
		groupIDs, err := r.loadPeerGroupIDs(ctx, p.ID)
		if err != nil {
			return nil, fmt.Errorf("load peer group IDs: %w", err)
		}
		p.GroupIDs = groupIDs

		out = append(out, &p)
	}
	return out, rows.Err()
}

// loadPeerGroupIDs loads all group IDs that a peer belongs to
func (r *NetworkRepository) loadPeerGroupIDs(ctx context.Context, peerID string) ([]string, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT group_id 
		FROM group_peers 
		WHERE peer_id = $1
		ORDER BY added_at ASC
	`, peerID)
	if err != nil {
		return nil, fmt.Errorf("query group IDs: %w", err)
	}
	defer func() { _ = rows.Close() }()

	groupIDs := make([]string, 0)
	for rows.Next() {
		var groupID string
		if err := rows.Scan(&groupID); err != nil {
			return nil, fmt.Errorf("scan group ID: %w", err)
		}
		groupIDs = append(groupIDs, groupID)
	}

	return groupIDs, rows.Err()
}

// nullableString converts an empty string to sql.NullString{Valid:false} so that
// optional text columns (cidr_v6, address_v6) store NULL instead of "".
func nullableString(s string) sql.NullString {
	if s == "" {
		return sql.NullString{}
	}
	return sql.NullString{String: s, Valid: true}
}

// ACL operations (ephemeral)
func (r *NetworkRepository) CreateACL(ctx context.Context, networkID string, acl *network.ACL) error {
	r.acls[networkID] = acl
	return nil
}
func (r *NetworkRepository) GetACL(ctx context.Context, networkID string) (*network.ACL, error) {
	return r.acls[networkID], nil
}
func (r *NetworkRepository) UpdateACL(ctx context.Context, networkID string, acl *network.ACL) error {
	r.acls[networkID] = acl
	return nil
}

// PeerConnection operations
func connectionKey(peer1ID, peer2ID string) (string, string) {
	if peer1ID < peer2ID {
		return peer1ID, peer2ID
	}
	return peer2ID, peer1ID
}

func (r *NetworkRepository) CreateConnection(ctx context.Context, networkID string, conn *network.PeerConnection) error {
	// Ensure peer order deterministic (peer1<peer2)
	p1, p2 := connectionKey(conn.Peer1ID, conn.Peer2ID)
	_, err := r.db.ExecContext(ctx, `INSERT INTO peer_connections (peer1_id,peer2_id,preshared_key,created_at) VALUES ($1,$2,$3,$4)`, p1, p2, conn.PresharedKey, time.Now())
	if err != nil {
		return fmt.Errorf("create connection: %w", err)
	}
	return nil
}

func (r *NetworkRepository) GetConnection(ctx context.Context, networkID, peer1ID, peer2ID string) (*network.PeerConnection, error) {
	p1, p2 := connectionKey(peer1ID, peer2ID)
	var c network.PeerConnection
	err := r.db.QueryRowContext(ctx, `SELECT peer1_id,peer2_id,preshared_key,created_at FROM peer_connections WHERE peer1_id=$1 AND peer2_id=$2`, p1, p2).
		Scan(&c.Peer1ID, &c.Peer2ID, &c.PresharedKey, &c.CreatedAt)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, fmt.Errorf("connection not found")
		}
		return nil, fmt.Errorf("get connection: %w", err)
	}
	return &c, nil
}

func (r *NetworkRepository) ListConnections(ctx context.Context, networkID string) ([]*network.PeerConnection, error) {
	// Filter by peers belonging to network using join
	rows, err := r.db.QueryContext(ctx, `SELECT c.peer1_id,c.peer2_id,c.preshared_key,c.created_at FROM peer_connections c
        JOIN peers p1 ON c.peer1_id=p1.id JOIN peers p2 ON c.peer2_id=p2.id WHERE p1.network_id=$1 AND p2.network_id=$1`, networkID)
	if err != nil {
		return nil, fmt.Errorf("list connections: %w", err)
	}
	defer func() {
		_ = rows.Close()
	}()
	out := make([]*network.PeerConnection, 0)
	for rows.Next() {
		var c network.PeerConnection
		if err = rows.Scan(&c.Peer1ID, &c.Peer2ID, &c.PresharedKey, &c.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, &c)
	}
	return out, rows.Err()
}

func (r *NetworkRepository) DeleteConnection(ctx context.Context, networkID, peer1ID, peer2ID string) error {
	p1, p2 := connectionKey(peer1ID, peer2ID)
	_, err := r.db.ExecContext(ctx, `DELETE FROM peer_connections WHERE peer1_id=$1 AND peer2_id=$2`, p1, p2)
	if err != nil {
		return fmt.Errorf("delete connection: %w", err)
	}
	return nil
}

// (IPAM operations removed - handled by dedicated IPAM repository)

// Agent session operations
func (r *NetworkRepository) CreateOrUpdateSession(ctx context.Context, networkID string, s *network.AgentSession) error {
	// Ensure peer belongs to network
	var exists bool
	err := r.db.QueryRowContext(ctx, `SELECT EXISTS (SELECT 1 FROM peers WHERE id=$1 AND network_id=$2)`, s.PeerID, networkID).Scan(&exists)
	if err != nil || !exists {
		return fmt.Errorf("peer not found in network")
	}
	now := time.Now()
	if s.FirstSeen.IsZero() {
		s.FirstSeen = now
	}
	s.LastSeen = now
	_, err = r.db.ExecContext(ctx, `INSERT INTO agent_sessions (session_id,peer_id,hostname,system_uptime,wireguard_uptime,reported_endpoint,last_seen,first_seen) VALUES ($1,$2,$3,$4,$5,$6,$7,$8)
        ON CONFLICT (session_id) DO UPDATE SET hostname=EXCLUDED.hostname,system_uptime=EXCLUDED.system_uptime,wireguard_uptime=EXCLUDED.wireguard_uptime,reported_endpoint=EXCLUDED.reported_endpoint,last_seen=EXCLUDED.last_seen`,
		s.SessionID, s.PeerID, s.Hostname, s.SystemUptime, s.WireGuardUptime, s.ReportedEndpoint, s.LastSeen, s.FirstSeen)
	if err != nil {
		return fmt.Errorf("upsert session: %w", err)
	}
	return nil
}

func (r *NetworkRepository) GetSession(ctx context.Context, networkID, peerID string) (*network.AgentSession, error) {
	// Return most recent session for peer
	var s network.AgentSession
	err := r.db.QueryRowContext(ctx, `SELECT session_id,peer_id,hostname,system_uptime,wireguard_uptime,reported_endpoint,last_seen,first_seen FROM agent_sessions WHERE peer_id=$1 ORDER BY last_seen DESC LIMIT 1`, peerID).
		Scan(&s.SessionID, &s.PeerID, &s.Hostname, &s.SystemUptime, &s.WireGuardUptime, &s.ReportedEndpoint, &s.LastSeen, &s.FirstSeen)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, fmt.Errorf("session not found")
		}
		return nil, fmt.Errorf("get session: %w", err)
	}
	// Validate peer belongs to network
	var belongs bool
	_ = r.db.QueryRowContext(ctx, `SELECT EXISTS (SELECT 1 FROM peers WHERE id=$1 AND network_id=$2)`, peerID, networkID).Scan(&belongs)
	if !belongs {
		return nil, fmt.Errorf("peer not in network")
	}
	return &s, nil
}

func (r *NetworkRepository) GetActiveSessionsForPeer(ctx context.Context, networkID, peerID string) ([]*network.AgentSession, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT session_id,peer_id,hostname,system_uptime,wireguard_uptime,reported_endpoint,last_seen,first_seen FROM agent_sessions WHERE peer_id=$1`, peerID)
	if err != nil {
		return nil, fmt.Errorf("list peer sessions: %w", err)
	}
	defer func() {
		_ = rows.Close()
	}()
	out := make([]*network.AgentSession, 0)
	var belongs bool
	_ = r.db.QueryRowContext(ctx, `SELECT EXISTS (SELECT 1 FROM peers WHERE id=$1 AND network_id=$2)`, peerID, networkID).Scan(&belongs)
	if !belongs {
		return nil, fmt.Errorf("peer not in network")
	}
	for rows.Next() {
		var s network.AgentSession
		if err = rows.Scan(&s.SessionID, &s.PeerID, &s.Hostname, &s.SystemUptime, &s.WireGuardUptime, &s.ReportedEndpoint, &s.LastSeen, &s.FirstSeen); err != nil {
			return nil, err
		}
		out = append(out, &s)
	}
	return out, rows.Err()
}

func (r *NetworkRepository) DeleteSession(ctx context.Context, networkID, sessionID string) error {
	_, err := r.db.ExecContext(ctx, `DELETE FROM agent_sessions WHERE session_id=$1`, sessionID)
	if err != nil {
		return fmt.Errorf("delete session: %w", err)
	}
	return nil
}

func (r *NetworkRepository) ListSessions(ctx context.Context, networkID string) ([]*network.AgentSession, error) {
	// Only sessions for peers in this network
	rows, err := r.db.QueryContext(ctx, `SELECT s.session_id,s.peer_id,s.hostname,s.system_uptime,s.wireguard_uptime,s.reported_endpoint,s.last_seen,s.first_seen FROM agent_sessions s
        JOIN peers p ON s.peer_id=p.id WHERE p.network_id=$1`, networkID)
	if err != nil {
		return nil, fmt.Errorf("list sessions: %w", err)
	}
	defer func() {
		_ = rows.Close()
	}()
	out := make([]*network.AgentSession, 0)
	for rows.Next() {
		var s network.AgentSession
		if err = rows.Scan(&s.SessionID, &s.PeerID, &s.Hostname, &s.SystemUptime, &s.WireGuardUptime, &s.ReportedEndpoint, &s.LastSeen, &s.FirstSeen); err != nil {
			return nil, err
		}
		out = append(out, &s)
	}
	return out, rows.Err()
}

// CaptivePortalWhitelistTTL is how long a whitelist entry remains valid after authentication.
// After this duration the peer must re-authenticate via the captive portal.
const CaptivePortalWhitelistTTL = 24 * time.Hour

// Captive portal whitelist operations

func (r *NetworkRepository) AddCaptivePortalWhitelist(ctx context.Context, networkID, jumpPeerID, peerIP, peerEndpoint string) error {
	now := time.Now()
	expiresAt := now.Add(CaptivePortalWhitelistTTL)
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO captive_portal_whitelist (network_id, jump_peer_id, peer_ip, peer_endpoint, created_at, expires_at)
		VALUES ($1, $2, $3, $4, $5, $6)
		ON CONFLICT (network_id, jump_peer_id, peer_ip)
		DO UPDATE SET expires_at = EXCLUDED.expires_at, peer_endpoint = EXCLUDED.peer_endpoint
	`, networkID, jumpPeerID, peerIP, peerEndpoint, now, expiresAt)
	return err
}

func (r *NetworkRepository) RemoveCaptivePortalWhitelist(ctx context.Context, networkID, jumpPeerID, peerIP string) error {
	_, err := r.db.ExecContext(ctx, `
		DELETE FROM captive_portal_whitelist
		WHERE network_id=$1 AND jump_peer_id=$2 AND peer_ip=$3
	`, networkID, jumpPeerID, peerIP)
	return err
}

// RemoveCaptivePortalWhitelistByPeerIP removes all whitelist entries for a peer IP across
// all jump peers in the network. Used when a security incident is detected (e.g. stolen config).
func (r *NetworkRepository) RemoveCaptivePortalWhitelistByPeerIP(ctx context.Context, networkID, peerIP string) error {
	_, err := r.db.ExecContext(ctx, `
		DELETE FROM captive_portal_whitelist
		WHERE network_id=$1 AND peer_ip=$2
	`, networkID, peerIP)
	return err
}

func (r *NetworkRepository) GetCaptivePortalWhitelist(ctx context.Context, networkID, jumpPeerID string) ([]string, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT peer_ip, COALESCE(peer_endpoint, '') FROM captive_portal_whitelist
		WHERE network_id=$1 AND jump_peer_id=$2
		  AND (expires_at IS NULL OR expires_at > NOW())
		ORDER BY created_at ASC
	`, networkID, jumpPeerID)
	if err != nil {
		return nil, err
	}
	defer func() {
		_ = rows.Close()
	}()

	var entries []string
	for rows.Next() {
		var wgIP, endpointIP string
		if err := rows.Scan(&wgIP, &endpointIP); err != nil {
			return nil, err
		}
		if endpointIP != "" {
			entries = append(entries, wgIP+"@"+endpointIP)
		} else {
			entries = append(entries, wgIP)
		}
	}
	return entries, rows.Err()
}

func (r *NetworkRepository) ClearCaptivePortalWhitelist(ctx context.Context, networkID, jumpPeerID string) error {
	_, err := r.db.ExecContext(ctx, `
		DELETE FROM captive_portal_whitelist
		WHERE network_id=$1 AND jump_peer_id=$2
	`, networkID, jumpPeerID)
	return err
}

func (r *NetworkRepository) CleanupExpiredCaptivePortalWhitelist(ctx context.Context) error {
	_, err := r.db.ExecContext(ctx, `
		DELETE FROM captive_portal_whitelist
		WHERE expires_at IS NOT NULL AND expires_at < NOW()
	`)
	return err
}

// Captive portal token operations

func (r *NetworkRepository) CreateCaptivePortalToken(ctx context.Context, token *network.CaptivePortalToken) error {
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO captive_portal_tokens (token, network_id, jump_peer_id, peer_ip, peer_endpoint, created_at, expires_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
	`, token.Token, token.NetworkID, token.JumpPeerID, token.PeerIP, token.PeerEndpoint, token.CreatedAt, token.ExpiresAt)
	return err
}

func (r *NetworkRepository) GetCaptivePortalToken(ctx context.Context, tokenStr string) (*network.CaptivePortalToken, error) {
	var token network.CaptivePortalToken
	var endpointIP sql.NullString
	err := r.db.QueryRowContext(ctx, `
		SELECT token, network_id, jump_peer_id, peer_ip, peer_endpoint, created_at, expires_at
		FROM captive_portal_tokens
		WHERE token=$1
	`, tokenStr).Scan(&token.Token, &token.NetworkID, &token.JumpPeerID, &token.PeerIP, &endpointIP, &token.CreatedAt, &token.ExpiresAt)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, fmt.Errorf("token not found")
		}
		return nil, fmt.Errorf("get captive portal token: %w", err)
	}
	if endpointIP.Valid {
		token.PeerEndpoint = endpointIP.String
	}
	return &token, nil
}

func (r *NetworkRepository) DeleteCaptivePortalToken(ctx context.Context, tokenStr string) error {
	_, err := r.db.ExecContext(ctx, `
		DELETE FROM captive_portal_tokens WHERE token=$1
	`, tokenStr)
	return err
}

func (r *NetworkRepository) CleanupExpiredCaptivePortalTokens(ctx context.Context) error {
	_, err := r.db.ExecContext(ctx, `
		DELETE FROM captive_portal_tokens WHERE expires_at < NOW()
	`)
	return err
}

func (r *NetworkRepository) MarkCaptivePortalTokenConsumed(ctx context.Context, tokenStr string) error {
	_, err := r.db.ExecContext(ctx, `
		UPDATE captive_portal_tokens SET consumed_at = NOW()
		WHERE token=$1 AND consumed_at IS NULL
	`, tokenStr)
	return err
}

func (r *NetworkRepository) ListExpiredUnconsumedCaptivePortalTokens(ctx context.Context) ([]*network.CaptivePortalToken, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT token, network_id, jump_peer_id, peer_ip, COALESCE(peer_endpoint, ''), created_at, expires_at
		FROM captive_portal_tokens
		WHERE expires_at < NOW() AND consumed_at IS NULL
	`)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	var out []*network.CaptivePortalToken
	for rows.Next() {
		t := &network.CaptivePortalToken{}
		if err := rows.Scan(&t.Token, &t.NetworkID, &t.JumpPeerID, &t.PeerIP, &t.PeerEndpoint, &t.CreatedAt, &t.ExpiresAt); err != nil {
			return nil, err
		}
		out = append(out, t)
	}
	return out, rows.Err()
}

// ListActiveCaptivePortalTokens returns all unexpired tokens for a jump peer.
func (r *NetworkRepository) ListActiveCaptivePortalTokens(ctx context.Context, networkID, jumpPeerID string) ([]*network.CaptivePortalToken, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT token, network_id, jump_peer_id, peer_ip, COALESCE(peer_endpoint, ''), created_at, expires_at
		FROM captive_portal_tokens
		WHERE network_id=$1 AND jump_peer_id=$2 AND expires_at > NOW()
		ORDER BY created_at ASC
	`, networkID, jumpPeerID)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	var out []*network.CaptivePortalToken
	for rows.Next() {
		t := &network.CaptivePortalToken{}
		if err := rows.Scan(&t.Token, &t.NetworkID, &t.JumpPeerID, &t.PeerIP, &t.PeerEndpoint, &t.CreatedAt, &t.ExpiresAt); err != nil {
			return nil, err
		}
		out = append(out, t)
	}
	return out, rows.Err()
}

// Endpoint denylist operations

func (r *NetworkRepository) AddEndpointDenylist(ctx context.Context, e *network.EndpointDenylistEntry) error {
	if e.CreatedAt.IsZero() {
		e.CreatedAt = time.Now()
	}
	if e.ExpiresAt.IsZero() {
		e.ExpiresAt = e.CreatedAt.Add(network.EndpointDenylistDefaultTTL)
	}
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO captive_portal_endpoint_denylist
			(network_id, jump_peer_id, wg_ip, blocked_ip, blocked_port, reason, created_at, expires_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		ON CONFLICT (network_id, jump_peer_id, wg_ip, blocked_ip, blocked_port)
		DO UPDATE SET reason = EXCLUDED.reason, expires_at = EXCLUDED.expires_at
	`, e.NetworkID, e.JumpPeerID, e.WgIP, e.BlockedIP, e.BlockedPort, e.Reason, e.CreatedAt, e.ExpiresAt)
	return err
}

func (r *NetworkRepository) GetEndpointDenylist(ctx context.Context, networkID, jumpPeerID string) ([]*network.EndpointDenylistEntry, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT network_id, jump_peer_id, wg_ip, blocked_ip, blocked_port, COALESCE(reason, ''), created_at, expires_at
		FROM captive_portal_endpoint_denylist
		WHERE network_id=$1 AND jump_peer_id=$2 AND expires_at > NOW()
		ORDER BY created_at ASC
	`, networkID, jumpPeerID)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	var out []*network.EndpointDenylistEntry
	for rows.Next() {
		e := &network.EndpointDenylistEntry{}
		if err := rows.Scan(&e.NetworkID, &e.JumpPeerID, &e.WgIP, &e.BlockedIP, &e.BlockedPort, &e.Reason, &e.CreatedAt, &e.ExpiresAt); err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	return out, rows.Err()
}

// ClearEndpointDenylistForPeer removes all denylist entries targeting the given
// peer (across all jump peers in the network).  Called when a peer successfully
// re-authenticates: their previous "rogue" source was actually a legitimate
// roam, and we should let them reconnect from any source.
func (r *NetworkRepository) ClearEndpointDenylistForPeer(ctx context.Context, networkID, wgIP string) error {
	_, err := r.db.ExecContext(ctx, `
		DELETE FROM captive_portal_endpoint_denylist
		WHERE network_id=$1 AND wg_ip=$2
	`, networkID, wgIP)
	return err
}

func (r *NetworkRepository) CleanupExpiredEndpointDenylist(ctx context.Context) error {
	_, err := r.db.ExecContext(ctx, `
		DELETE FROM captive_portal_endpoint_denylist
		WHERE expires_at < NOW()
	`)
	return err
}

// Quarantine operations

func (r *NetworkRepository) GetQuarantine(ctx context.Context, networkID, peerID string) (*network.CaptivePortalQuarantine, error) {
	q := &network.CaptivePortalQuarantine{NetworkID: networkID, PeerID: peerID}
	var lastStrike, until sql.NullTime
	err := r.db.QueryRowContext(ctx, `
		SELECT strikes, last_strike_at, quarantined_until
		FROM captive_portal_quarantine
		WHERE network_id=$1 AND peer_id=$2
	`, networkID, peerID).Scan(&q.Strikes, &lastStrike, &until)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("get quarantine: %w", err)
	}
	if lastStrike.Valid {
		t := lastStrike.Time
		q.LastStrikeAt = &t
	}
	if until.Valid {
		t := until.Time
		q.QuarantinedUntil = &t
	}
	return q, nil
}

func (r *NetworkRepository) UpsertQuarantine(ctx context.Context, q *network.CaptivePortalQuarantine) error {
	var lastStrike, until interface{}
	if q.LastStrikeAt != nil {
		lastStrike = *q.LastStrikeAt
	}
	if q.QuarantinedUntil != nil {
		until = *q.QuarantinedUntil
	}
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO captive_portal_quarantine (network_id, peer_id, strikes, last_strike_at, quarantined_until)
		VALUES ($1, $2, $3, $4, $5)
		ON CONFLICT (network_id, peer_id)
		DO UPDATE SET strikes=$3, last_strike_at=$4, quarantined_until=$5
	`, q.NetworkID, q.PeerID, q.Strikes, lastStrike, until)
	return err
}

func (r *NetworkRepository) ListQuarantinedPeers(ctx context.Context, networkID string) ([]*network.CaptivePortalQuarantine, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT network_id, peer_id, strikes, last_strike_at, quarantined_until
		FROM captive_portal_quarantine
		WHERE network_id=$1 AND quarantined_until IS NOT NULL AND quarantined_until > NOW()
		ORDER BY peer_id
	`, networkID)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	var out []*network.CaptivePortalQuarantine
	for rows.Next() {
		q := &network.CaptivePortalQuarantine{}
		var lastStrike, until sql.NullTime
		if err := rows.Scan(&q.NetworkID, &q.PeerID, &q.Strikes, &lastStrike, &until); err != nil {
			return nil, err
		}
		if lastStrike.Valid {
			t := lastStrike.Time
			q.LastStrikeAt = &t
		}
		if until.Valid {
			t := until.Time
			q.QuarantinedUntil = &t
		}
		out = append(out, q)
	}
	return out, rows.Err()
}

func (r *NetworkRepository) ClearQuarantine(ctx context.Context, networkID, peerID string) error {
	_, err := r.db.ExecContext(ctx, `
		DELETE FROM captive_portal_quarantine
		WHERE network_id=$1 AND peer_id=$2
	`, networkID, peerID)
	return err
}

// Per-peer local routes (reported via heartbeat)

func (r *NetworkRepository) UpsertPeerLocalRoutes(ctx context.Context, networkID, peerID string, allowedIPs []string) error {
	if allowedIPs == nil {
		allowedIPs = []string{}
	}
	data, err := json.Marshal(allowedIPs)
	if err != nil {
		return fmt.Errorf("marshal allowed_ips: %w", err)
	}
	_, err = r.db.ExecContext(ctx, `
		INSERT INTO peer_local_routes (network_id, peer_id, allowed_ips, updated_at)
		VALUES ($1, $2, $3, NOW())
		ON CONFLICT (network_id, peer_id)
		DO UPDATE SET allowed_ips=EXCLUDED.allowed_ips, updated_at=NOW()
	`, networkID, peerID, string(data))
	return err
}

func (r *NetworkRepository) GetPeerLocalRoutes(ctx context.Context, networkID, peerID string) ([]string, error) {
	var raw string
	err := r.db.QueryRowContext(ctx, `
		SELECT allowed_ips FROM peer_local_routes
		WHERE network_id=$1 AND peer_id=$2
	`, networkID, peerID).Scan(&raw)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("get peer_local_routes: %w", err)
	}
	var out []string
	if err := json.Unmarshal([]byte(raw), &out); err != nil {
		return nil, fmt.Errorf("unmarshal allowed_ips: %w", err)
	}
	return out, nil
}

func (r *NetworkRepository) ListPeerLocalRoutes(ctx context.Context, networkID string) (map[string][]string, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT peer_id, allowed_ips FROM peer_local_routes
		WHERE network_id=$1
	`, networkID)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	out := make(map[string][]string)
	for rows.Next() {
		var peerID, raw string
		if err := rows.Scan(&peerID, &raw); err != nil {
			return nil, err
		}
		var cidrs []string
		if err := json.Unmarshal([]byte(raw), &cidrs); err == nil {
			out[peerID] = cidrs
		}
	}
	return out, rows.Err()
}

