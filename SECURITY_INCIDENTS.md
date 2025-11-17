# Security Incident Tracking System

## Backend Implementation

### Domain Models

#### SecurityIncident (`internal/domain/network/session.go`)
```go
type SecurityIncident struct {
    ID           string    // Unique incident identifier
    PeerID       string    // Affected peer ID
    PeerName     string    // Peer name for display
    NetworkID    string    // Network where incident occurred
    NetworkName  string    // Network name for display
    IncidentType string    // "shared_config", "session_conflict", "suspicious_activity"
    DetectedAt   time.Time // When the incident was detected
    PublicKey    string    // The public key involved
    Endpoints    []string  // List of endpoints involved
    Details      string    // Human-readable description
    Resolved     bool      // Resolution status
    ResolvedAt   time.Time // When resolved
    ResolvedBy   string    // Who resolved it
}
```

### API Endpoints

#### Security Incidents
- `GET /api/v1/security/incidents` - List all security incidents (with optional `resolved` filter)
- `GET /api/v1/security/incidents/:incidentId` - Get specific incident details
- `POST /api/v1/security/incidents/:incidentId/resolve` - Mark incident as resolved
- `GET /api/v1/networks/:networkId/security/incidents` - List incidents for a network

#### Peer Management
- `POST /api/v1/networks/:networkId/peers/:peerId/reconnect` - Reconnect peer to all jump servers
- `GET /api/v1/networks/:networkId/peers/:peerId/session` - Get peer session status

### Automatic Incident Creation

When the system detects shared configurations (same public key with different endpoints within 5 minutes), it:

1. **Creates a SecurityIncident** for each affected peer
2. **Removes peer connections** from all jump servers (blocks access)
3. **Logs the event** with detailed information

### Repository Methods

```go
CreateSecurityIncident(ctx, incident) error
GetSecurityIncident(ctx, incidentID) (*SecurityIncident, error)
ListSecurityIncidents(ctx, resolved *bool) ([]*SecurityIncident, error)
ListSecurityIncidentsByNetwork(ctx, networkID, resolved *bool) ([]*SecurityIncident, error)
ResolveSecurityIncident(ctx, incidentID, resolvedBy) error
```

### Service Methods

```go
ListSecurityIncidents(ctx, resolved *bool) ([]*SecurityIncident, error)
ListSecurityIncidentsByNetwork(ctx, networkID, resolved *bool) ([]*SecurityIncident, error)
GetSecurityIncident(ctx, incidentID) (*SecurityIncident, error)
ResolveSecurityIncident(ctx, incidentID, resolvedBy) error
ReconnectPeer(ctx, networkID, peerID) error
```

## Frontend Implementation

### New Screens

1. **SecurityIncidentListScreen** (`src/screens/security/SecurityIncidentListScreen.tsx`)
   - Lists all security incidents
   - Filter by: Active / All / Resolved
   - Search by peer name, network name, or public key
   - Actions: View Peer, Reconnect

2. **PeerSecurityDetailsScreen** (`src/screens/security/PeerSecurityDetailsScreen.tsx`)
   - Shows security status for a specific peer
   - Current session information
   - Conflicting sessions (if any)
   - Recent endpoint changes
   - Suspicious activity alerts

### Navigation

- New "Security" tab with shield-alert icon
- Available in both mobile and web interfaces
- Accessible from main navigation

### Enhanced Peer View

Peer details now show security status badges:
- ✅ **Connected** (green) - Active agent
- ⚠️ **Security Alert** (orange-red) - Suspicious activity
- 🔴 **Session Conflict** (red) - Multiple sessions detected

### API Client Methods

```typescript
getPeerSessionStatus(networkId, peerId): Promise<PeerSessionStatus>
getNetworkSessions(networkId): Promise<PeerSessionStatus[]>
getSecurityIncidents(resolved?): Promise<SecurityIncident[]>
resolveSecurityIncident(incidentId): Promise<void>
reconnectPeer(networkId, peerId): Promise<void>
```

## Workflow Example

### Shared Configuration Detected

1. **Agent A** connects with peer public key `ABC123...` from endpoint `192.168.1.10:51820`
2. **Agent B** connects with the same public key from endpoint `192.168.2.20:51820`
3. **System detects** the same public key at different endpoints within 5 minutes
4. **Incident created** for both agents with type `shared_config`
5. **Both peers removed** from all jump servers (connections deleted)
6. **Frontend displays** incidents in Security tab

### Administrator Response

1. Navigate to **Security > Incidents**
2. See incident: "Public key ABC1... detected at 2 different endpoints"
3. View affected peers and endpoints
4. Investigate the issue
5. Click **Reconnect** to restore peer access to jump servers
6. Incident automatically marked as resolved

## Security Benefits

- **Automatic detection** of configuration sharing
- **Immediate response** by blocking access
- **Audit trail** of all security incidents
- **Easy remediation** with one-click reconnect
- **Visibility** into peer security status
- **Prevention** of unauthorized access through shared configs
