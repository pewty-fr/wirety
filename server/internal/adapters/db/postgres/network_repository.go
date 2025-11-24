package postgres

import (
	"context"
	"database/sql"
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
	_, err := r.db.ExecContext(ctx, `INSERT INTO networks (id,name,cidr,dns,created_at,updated_at) VALUES ($1,$2,$3,$4,$5,$6)`,
		n.ID, n.Name, n.CIDR, pq.Array(n.DNS), n.CreatedAt, n.UpdatedAt)
	if err != nil {
		return fmt.Errorf("create network: %w", err)
	}

	// Store the ACL in memory if it exists
	if n.ACL != nil {
		r.acls[n.ID] = n.ACL
	}

	return nil
}

func (r *NetworkRepository) GetNetwork(ctx context.Context, networkID string) (*network.Network, error) {
	var n network.Network
	err := r.db.QueryRowContext(ctx, `SELECT id,name,cidr,dns,created_at,updated_at FROM networks WHERE id=$1`, networkID).
		Scan(&n.ID, &n.Name, &n.CIDR, pq.Array(&n.DNS), &n.CreatedAt, &n.UpdatedAt)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, fmt.Errorf("network not found")
		}
		return nil, fmt.Errorf("get network: %w", err)
	}
	// Load peers
	n.Peers = make(map[string]*network.Peer)
	rows, err := r.db.QueryContext(ctx, `SELECT id,name,public_key,private_key,address,endpoint,listen_port,additional_allowed_ips,token,is_jump,is_isolated,full_encapsulation,use_agent,owner_id,created_at,updated_at FROM peers WHERE network_id=$1`, networkID)
	if err != nil {
		return nil, fmt.Errorf("load peers: %w", err)
	}
	defer rows.Close()
	count := 0
	for rows.Next() {
		var p network.Peer
		var addrs []string
		err = rows.Scan(&p.ID, &p.Name, &p.PublicKey, &p.PrivateKey, &p.Address, &p.Endpoint, &p.ListenPort, pq.Array(&addrs), &p.Token, &p.IsJump, &p.IsIsolated, &p.FullEncapsulation, &p.UseAgent, &p.OwnerID, &p.CreatedAt, &p.UpdatedAt)
		if err != nil {
			return nil, fmt.Errorf("scan peer: %w", err)
		}
		p.AdditionalAllowedIPs = addrs
		n.AddPeer(&p)
		count++
	}
	n.PeerCount = count
	// Reattach ACL if present (ephemeral)
	n.ACL = r.acls[networkID]
	return &n, nil
}

func (r *NetworkRepository) UpdateNetwork(ctx context.Context, n *network.Network) error {
	n.UpdatedAt = time.Now()
	_, err := r.db.ExecContext(ctx, `UPDATE networks SET name=$2,cidr=$3,dns=$4,updated_at=$5 WHERE id=$1`, n.ID, n.Name, n.CIDR, pq.Array(n.DNS), n.UpdatedAt)
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
	rows, err := r.db.QueryContext(ctx, `SELECT n.id,n.name,n.cidr,n.dns,n.created_at,n.updated_at, COALESCE(p.peer_count,0) AS peer_count FROM networks n LEFT JOIN (SELECT network_id, COUNT(*) AS peer_count FROM peers GROUP BY network_id) p ON p.network_id = n.id ORDER BY n.created_at ASC`)
	if err != nil {
		return nil, fmt.Errorf("list networks: %w", err)
	}
	defer rows.Close()
	out := make([]*network.Network, 0)
	for rows.Next() {
		var n network.Network
		err = rows.Scan(&n.ID, &n.Name, &n.CIDR, pq.Array(&n.DNS), &n.CreatedAt, &n.UpdatedAt, &n.PeerCount)
		if err != nil {
			return nil, err
		}
		n.Peers = make(map[string]*network.Peer) // not loaded to keep call light
		n.ACL = r.acls[n.ID]
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
	_, err := r.db.ExecContext(ctx, `INSERT INTO peers (id,network_id,name,public_key,private_key,address,endpoint,listen_port,additional_allowed_ips,token,is_jump,is_isolated,full_encapsulation,use_agent,owner_id,created_at,updated_at) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17)`,
		p.ID, networkID, p.Name, p.PublicKey, p.PrivateKey, p.Address, p.Endpoint, p.ListenPort, pq.Array(p.AdditionalAllowedIPs), p.Token, p.IsJump, p.IsIsolated, p.FullEncapsulation, p.UseAgent, p.OwnerID, p.CreatedAt, p.UpdatedAt)
	if err != nil {
		return fmt.Errorf("create peer: %w", err)
	}
	return nil
}

func (r *NetworkRepository) GetPeer(ctx context.Context, networkID, peerID string) (*network.Peer, error) {
	var p network.Peer
	var addrs []string
	err := r.db.QueryRowContext(ctx, `SELECT id,name,public_key,private_key,address,endpoint,listen_port,additional_allowed_ips,token,is_jump,is_isolated,full_encapsulation,use_agent,owner_id,created_at,updated_at FROM peers WHERE id=$1 AND network_id=$2`, peerID, networkID).
		Scan(&p.ID, &p.Name, &p.PublicKey, &p.PrivateKey, &p.Address, &p.Endpoint, &p.ListenPort, pq.Array(&addrs), &p.Token, &p.IsJump, &p.IsIsolated, &p.FullEncapsulation, &p.UseAgent, &p.OwnerID, &p.CreatedAt, &p.UpdatedAt)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, fmt.Errorf("peer not found")
		}
		return nil, fmt.Errorf("get peer: %w", err)
	}
	p.AdditionalAllowedIPs = addrs
	return &p, nil
}

func (r *NetworkRepository) GetPeerByToken(ctx context.Context, token string) (string, *network.Peer, error) {
	var p network.Peer
	var networkID string
	var addrs []string
	err := r.db.QueryRowContext(ctx, `SELECT network_id,id,name,public_key,private_key,address,endpoint,listen_port,additional_allowed_ips,token,is_jump,is_isolated,full_encapsulation,use_agent,owner_id,created_at,updated_at FROM peers WHERE token=$1`, token).
		Scan(&networkID, &p.ID, &p.Name, &p.PublicKey, &p.PrivateKey, &p.Address, &p.Endpoint, &p.ListenPort, pq.Array(&addrs), &p.Token, &p.IsJump, &p.IsIsolated, &p.FullEncapsulation, &p.UseAgent, &p.OwnerID, &p.CreatedAt, &p.UpdatedAt)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return "", nil, fmt.Errorf("token not found")
		}
		return "", nil, fmt.Errorf("get peer by token: %w", err)
	}
	p.AdditionalAllowedIPs = addrs
	return networkID, &p, nil
}

func (r *NetworkRepository) UpdatePeer(ctx context.Context, networkID string, p *network.Peer) error {
	p.UpdatedAt = time.Now()
	// Ensure AdditionalAllowedIPs is never nil to avoid database constraint violation
	if p.AdditionalAllowedIPs == nil {
		p.AdditionalAllowedIPs = []string{}
	}
	res, err := r.db.ExecContext(ctx, `UPDATE peers SET name=$3,public_key=$4,private_key=$5,address=$6,endpoint=$7,listen_port=$8,additional_allowed_ips=$9,token=$10,is_jump=$11,is_isolated=$12,full_encapsulation=$13,use_agent=$14,owner_id=$15,updated_at=$16 WHERE id=$1 AND network_id=$2`,
		p.ID, networkID, p.Name, p.PublicKey, p.PrivateKey, p.Address, p.Endpoint, p.ListenPort, pq.Array(p.AdditionalAllowedIPs), p.Token, p.IsJump, p.IsIsolated, p.FullEncapsulation, p.UseAgent, p.OwnerID, p.UpdatedAt)
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
	rows, err := r.db.QueryContext(ctx, `SELECT id,name,public_key,private_key,address,endpoint,listen_port,additional_allowed_ips,token,is_jump,is_isolated,full_encapsulation,use_agent,owner_id,created_at,updated_at FROM peers WHERE network_id=$1 ORDER BY created_at ASC`, networkID)
	if err != nil {
		return nil, fmt.Errorf("list peers: %w", err)
	}
	defer rows.Close()
	out := make([]*network.Peer, 0)
	for rows.Next() {
		var p network.Peer
		var addrs []string
		err = rows.Scan(&p.ID, &p.Name, &p.PublicKey, &p.PrivateKey, &p.Address, &p.Endpoint, &p.ListenPort, pq.Array(&addrs), &p.Token, &p.IsJump, &p.IsIsolated, &p.FullEncapsulation, &p.UseAgent, &p.OwnerID, &p.CreatedAt, &p.UpdatedAt)
		if err != nil {
			return nil, err
		}
		p.AdditionalAllowedIPs = addrs
		out = append(out, &p)
	}
	return out, rows.Err()
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
	defer rows.Close()
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
	defer rows.Close()
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
	defer rows.Close()
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

// Endpoint change tracking
func (r *NetworkRepository) RecordEndpointChange(ctx context.Context, networkID string, change *network.EndpointChange) error {
	_, err := r.db.ExecContext(ctx, `INSERT INTO endpoint_changes (peer_id,old_endpoint,new_endpoint,changed_at,source) VALUES ($1,$2,$3,$4,$5)`,
		change.PeerID, change.OldEndpoint, change.NewEndpoint, change.ChangedAt, change.Source)
	if err != nil {
		return fmt.Errorf("record endpoint change: %w", err)
	}
	return nil
}

func (r *NetworkRepository) GetEndpointChanges(ctx context.Context, networkID, peerID string, since time.Time) ([]*network.EndpointChange, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT peer_id,old_endpoint,new_endpoint,changed_at,source FROM endpoint_changes WHERE peer_id=$1 AND changed_at > $2 ORDER BY changed_at DESC`, peerID, since)
	if err != nil {
		return nil, fmt.Errorf("get endpoint changes: %w", err)
	}
	defer rows.Close()
	out := make([]*network.EndpointChange, 0)
	for rows.Next() {
		var c network.EndpointChange
		if err = rows.Scan(&c.PeerID, &c.OldEndpoint, &c.NewEndpoint, &c.ChangedAt, &c.Source); err != nil {
			return nil, err
		}
		out = append(out, &c)
	}
	return out, rows.Err()
}

func (r *NetworkRepository) DeleteEndpointChanges(ctx context.Context, networkID, peerID string) error {
	_, err := r.db.ExecContext(ctx, `DELETE FROM endpoint_changes WHERE peer_id=$1`, peerID)
	if err != nil {
		return fmt.Errorf("delete endpoint changes: %w", err)
	}
	return nil
}

// Security incident operations
func (r *NetworkRepository) CreateSecurityIncident(ctx context.Context, incident *network.SecurityIncident) error {
	_, err := r.db.ExecContext(ctx, `INSERT INTO security_incidents (id,peer_id,peer_name,network_id,network_name,incident_type,detected_at,public_key,endpoints,details,resolved,resolved_at,resolved_by) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13)`,
		incident.ID, incident.PeerID, incident.PeerName, incident.NetworkID, incident.NetworkName, incident.IncidentType, incident.DetectedAt, incident.PublicKey, pq.Array(incident.Endpoints), incident.Details, incident.Resolved, nullTimePtr(incident.ResolvedAt), incident.ResolvedBy)
	if err != nil {
		return fmt.Errorf("create incident: %w", err)
	}
	return nil
}

func (r *NetworkRepository) GetSecurityIncident(ctx context.Context, incidentID string) (*network.SecurityIncident, error) {
	var i network.SecurityIncident
	var endpoints []string
	var resolvedAt sql.NullTime
	err := r.db.QueryRowContext(ctx, `SELECT id,peer_id,peer_name,network_id,network_name,incident_type,detected_at,public_key,endpoints,details,resolved,resolved_at,resolved_by FROM security_incidents WHERE id=$1`, incidentID).
		Scan(&i.ID, &i.PeerID, &i.PeerName, &i.NetworkID, &i.NetworkName, &i.IncidentType, &i.DetectedAt, &i.PublicKey, pq.Array(&endpoints), &i.Details, &i.Resolved, &resolvedAt, &i.ResolvedBy)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, fmt.Errorf("incident not found")
		}
		return nil, fmt.Errorf("get incident: %w", err)
	}
	i.Endpoints = endpoints
	if resolvedAt.Valid {
		i.ResolvedAt = resolvedAt.Time
	}
	return &i, nil
}

func (r *NetworkRepository) ListSecurityIncidents(ctx context.Context, resolved *bool) ([]*network.SecurityIncident, error) {
	q := `SELECT id,peer_id,peer_name,network_id,network_name,incident_type,detected_at,public_key,endpoints,details,resolved,resolved_at,resolved_by FROM security_incidents`
	args := []interface{}{}
	if resolved != nil {
		q += ` WHERE resolved=$1`
		args = append(args, *resolved)
	}
	rows, err := r.db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, fmt.Errorf("list incidents: %w", err)
	}
	defer rows.Close()
	out := make([]*network.SecurityIncident, 0)
	for rows.Next() {
		var i network.SecurityIncident
		var endpoints []string
		var resolvedAt sql.NullTime
		if err = rows.Scan(&i.ID, &i.PeerID, &i.PeerName, &i.NetworkID, &i.NetworkName, &i.IncidentType, &i.DetectedAt, &i.PublicKey, pq.Array(&endpoints), &i.Details, &i.Resolved, &resolvedAt, &i.ResolvedBy); err != nil {
			return nil, err
		}
		i.Endpoints = endpoints
		if resolvedAt.Valid {
			i.ResolvedAt = resolvedAt.Time
		}
		out = append(out, &i)
	}
	return out, rows.Err()
}

func (r *NetworkRepository) ListSecurityIncidentsByNetwork(ctx context.Context, networkID string, resolved *bool) ([]*network.SecurityIncident, error) {
	q := `SELECT id,peer_id,peer_name,network_id,network_name,incident_type,detected_at,public_key,endpoints,details,resolved,resolved_at,resolved_by FROM security_incidents WHERE network_id=$1`
	args := []interface{}{networkID}
	if resolved != nil {
		q += ` AND resolved=$2`
		args = append(args, *resolved)
	}
	rows, err := r.db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, fmt.Errorf("list network incidents: %w", err)
	}
	defer rows.Close()
	out := make([]*network.SecurityIncident, 0)
	for rows.Next() {
		var i network.SecurityIncident
		var endpoints []string
		var resolvedAt sql.NullTime
		if err = rows.Scan(&i.ID, &i.PeerID, &i.PeerName, &i.NetworkID, &i.NetworkName, &i.IncidentType, &i.DetectedAt, &i.PublicKey, pq.Array(&endpoints), &i.Details, &i.Resolved, &resolvedAt, &i.ResolvedBy); err != nil {
			return nil, err
		}
		i.Endpoints = endpoints
		if resolvedAt.Valid {
			i.ResolvedAt = resolvedAt.Time
		}
		out = append(out, &i)
	}
	return out, rows.Err()
}

func (r *NetworkRepository) ResolveSecurityIncident(ctx context.Context, incidentID, resolvedBy string) error {
	_, err := r.db.ExecContext(ctx, `UPDATE security_incidents SET resolved=TRUE,resolved_at=$2,resolved_by=$3 WHERE id=$1`, incidentID, time.Now(), resolvedBy)
	if err != nil {
		return fmt.Errorf("resolve incident: %w", err)
	}
	return nil
}
