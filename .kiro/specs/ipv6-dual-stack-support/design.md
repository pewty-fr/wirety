# Design Document

## Overview

This design document describes the implementation of flexible IP stack support for the Wirety WireGuard network management system. The system will support three IP stack modes: IPv4-only, IPv6-only, and dual-stack (IPv4 + IPv6). This enables users to choose the appropriate IP configuration based on their infrastructure requirements while maintaining backward compatibility with existing IPv4-only networks.

The design follows the existing hexagonal architecture pattern and extends the current IPAM, network management, policy enforcement, and DNS systems to handle multiple IP versions.

## Architecture

### High-Level Architecture

```
┌─────────────────────────────────────────────────────────────┐
│                        Frontend UI                          │
│  - IP Stack Mode Selector (IPv4/IPv6/Dual)                 │
│  - IPv4 & IPv6 CIDR Input Fields                           │
│  - Display Multiple IP Addresses per Peer                  │
└─────────────────────────────────────────────────────────────┘
                            │
                            ▼
┌─────────────────────────────────────────────────────────────┐
│                      API Layer (REST)                       │
│  - Network CRUD with IP Stack Mode                         │
│  - Peer Management with Multi-IP Support                   │
│  - Policy Rules with IP Version Detection                  │
└─────────────────────────────────────────────────────────────┘
                            │
                            ▼
┌─────────────────────────────────────────────────────────────┐
│                   Application Services                      │
│  ┌─────────────────────────────────────────────────────┐   │
│  │ NetworkService                                      │   │
│  │  - IP Stack Mode Validation                        │   │
│  │  - Multi-IP Peer Creation                          │   │
│  │  - WireGuard Config Generation (IPv4/IPv6/Both)    │   │
│  └─────────────────────────────────────────────────────┘   │
│  ┌─────────────────────────────────────────────────────┐   │
│  │ IPAMService                                         │   │
│  │  - IPv4 Address Allocation                         │   │
│  │  - IPv6 Address Allocation (NEW)                   │   │
│  │  - Dual-Stack Address Management                   │   │
│  └─────────────────────────────────────────────────────┘   │
│  ┌─────────────────────────────────────────────────────┐   │
│  │ PolicyService                                       │   │
│  │  - IP Version Detection from CIDR                  │   │
│  │  - iptables Rule Generation (IPv4)                 │   │
│  │  - ip6tables Rule Generation (IPv6) (NEW)          │   │
│  └─────────────────────────────────────────────────────┘   │
│  ┌─────────────────────────────────────────────────────┐   │
│  │ DNSService                                          │   │
  │  - A Record Management (IPv4)                      │   │
│  │  - AAAA Record Management (IPv6) (NEW)             │   │
│  └─────────────────────────────────────────────────────┘   │
└─────────────────────────────────────────────────────────────┘
                            │
                            ▼
┌─────────────────────────────────────────────────────────────┐
│                    Domain Models                            │
│  - Network: IPStackMode, IPv4CIDR, IPv6CIDR               │
│  - Peer: IPv4Address, IPv6Address                          │
│  - PolicyRule: IPVersion (auto-detected)                   │
│  - Route: DestinationCIDR (IPv4 or IPv6)                  │
└─────────────────────────────────────────────────────────────┘
                            │
                            ▼
┌─────────────────────────────────────────────────────────────┐
│                   Repository Layer                          │
│  - PostgreSQL: Store IP Stack Mode & Multiple IPs         │
│  - IPAM Repository: IPv4 & IPv6 Prefix Management         │
└─────────────────────────────────────────────────────────────┘
                            │
                            ▼
┌─────────────────────────────────────────────────────────────┐
│                    Agent (Jump Peer)                        │
│  - Firewall Adapter: iptables + ip6tables                 │
│  - DNS Server: A + AAAA Records                            │
│  - Kernel: IPv4 & IPv6 Forwarding                         │
└─────────────────────────────────────────────────────────────┘
```

### IP Stack Mode Flow

```
Network Creation
      │
      ▼
┌─────────────────┐
│ Select IP Stack │
│ Mode            │
└─────────────────┘
      │
      ├─────────────┬─────────────┬─────────────┐
      ▼             ▼             ▼             ▼
  IPv4 Only    IPv6 Only    Dual-Stack    (Default: IPv4)
      │             │             │
      ▼             ▼             ▼
 Require       Require       Require
 IPv4 CIDR     IPv6 CIDR     Both CIDRs
      │             │             │
      ▼             ▼             ▼
 Allocate      Allocate      Allocate
 IPv4 Only     IPv6 Only     Both IPs
      │             │             │
      ▼             ▼             ▼
 Generate      Generate      Generate
 iptables      ip6tables     Both Rules
```

## Components and Interfaces

### 1. Network Domain Model Extensions

```go
// IPStackMode represents the IP protocol configuration for a network
type IPStackMode string

const (
    IPStackModeIPv4 IPStackMode = "ipv4"
    IPStackModeIPv6 IPStackMode = "ipv6"
    IPStackModeDual IPStackMode = "dual"
)

// Network represents a WireGuard network
type Network struct {
    ID              string
    Name            string
    IPStackMode     IPStackMode  // NEW: IP stack configuration
    IPv4CIDR        string       // RENAMED from CIDR
    IPv6CIDR        string       // NEW: IPv6 network range
    Peers           map[string]*Peer
    DomainSuffix    string
    DefaultGroupIDs []string
    DNS             []string
    CreatedAt       time.Time
    UpdatedAt       time.Time
}

// NetworkCreateRequest for creating a new network
type NetworkCreateRequest struct {
    Name         string
    IPStackMode  IPStackMode  // NEW
    IPv4CIDR     string       // Optional based on mode
    IPv6CIDR     string       // Optional based on mode
    DomainSuffix string
    DNS          []string
}

// Validate checks if the network configuration is valid
func (r *NetworkCreateRequest) Validate() error {
    // Validate IP stack mode
    if r.IPStackMode == "" {
        r.IPStackMode = IPStackModeIPv4 // Default for backward compatibility
    }
    
    switch r.IPStackMode {
    case IPStackModeIPv4:
        if r.IPv4CIDR == "" {
            return errors.New("IPv4 CIDR required for IPv4-only mode")
        }
        if r.IPv6CIDR != "" {
            return errors.New("IPv6 CIDR not allowed in IPv4-only mode")
        }
    case IPStackModeIPv6:
        if r.IPv6CIDR == "" {
            return errors.New("IPv6 CIDR required for IPv6-only mode")
        }
        if r.IPv4CIDR != "" {
            return errors.New("IPv4 CIDR not allowed in IPv6-only mode")
        }
    case IPStackModeDual:
        if r.IPv4CIDR == "" || r.IPv6CIDR == "" {
            return errors.New("both IPv4 and IPv6 CIDRs required for dual-stack mode")
        }
    default:
        return errors.New("invalid IP stack mode")
    }
    
    return nil
}
```

### 2. Peer Domain Model Extensions

```go
// Peer represents a WireGuard peer
type Peer struct {
    ID                   string
    Name                 string
    PublicKey            string
    PrivateKey           string
    IPv4Address          string   // RENAMED from Address
    IPv6Address          string   // NEW
    Endpoint             string
    ListenPort           int
    IsJump               bool
    UseAgent             bool
    AdditionalAllowedIPs []string
    OwnerID              string
    GroupIDs             []string
    Token                string
    CreatedAt            time.Time
    UpdatedAt            time.Time
}

// GetAddresses returns all IP addresses for the peer
func (p *Peer) GetAddresses() []string {
    var addrs []string
    if p.IPv4Address != "" {
        addrs = append(addrs, p.IPv4Address)
    }
    if p.IPv6Address != "" {
        addrs = append(addrs, p.IPv6Address)
    }
    return addrs
}

// HasIPv4 returns true if peer has an IPv4 address
func (p *Peer) HasIPv4() bool {
    return p.IPv4Address != ""
}

// HasIPv6 returns true if peer has an IPv6 address
func (p *Peer) HasIPv6() bool {
    return p.IPv6Address != ""
}
```

### 3. IPAM Service Extensions

```go
// IPAMService manages IP address allocation for both IPv4 and IPv6
type IPAMService interface {
    // IPv4 methods (existing)
    AcquireIPv4(ctx context.Context, cidr string) (string, error)
    ReleaseIPv4(ctx context.Context, cidr, ip string) error
    
    // IPv6 methods (new)
    AcquireIPv6(ctx context.Context, cidr string) (string, error)
    ReleaseIPv6(ctx context.Context, cidr, ip string) error
    
    // Dual-stack methods (new)
    AcquireDualStack(ctx context.Context, ipv4CIDR, ipv6CIDR string) (ipv4, ipv6 string, err error)
    ReleaseDualStack(ctx context.Context, ipv4CIDR, ipv4, ipv6CIDR, ipv6 string) error
    
    // Utility methods
    EnsureIPv6RootPrefix(ctx context.Context, cidr string) (*Prefix, error)
    IsIPv6CIDR(cidr string) bool
}
```

### 4. Policy Service Extensions

```go
// PolicyService generates firewall rules for both IPv4 and IPv6
type PolicyService interface {
    // Existing methods
    CreatePolicy(ctx context.Context, networkID string, req *PolicyCreateRequest) (*Policy, error)
    GetPolicy(ctx context.Context, networkID, policyID string) (*Policy, error)
    UpdatePolicy(ctx context.Context, networkID, policyID string, req *PolicyUpdateRequest) (*Policy, error)
    DeletePolicy(ctx context.Context, networkID, policyID string) error
    ListPolicies(ctx context.Context, networkID string) ([]*Policy, error)
    
    // Extended methods
    GenerateIPTablesRules(ctx context.Context, networkID, jumpPeerID string) ([]string, error)
    GenerateIP6TablesRules(ctx context.Context, networkID, jumpPeerID string) ([]string, error) // NEW
    
    // Utility methods
    DetectIPVersion(cidr string) (IPVersion, error) // NEW
}

// IPVersion represents the IP protocol version
type IPVersion int

const (
    IPVersionIPv4 IPVersion = 4
    IPVersionIPv6 IPVersion = 6
)

// DetectIPVersion determines if a CIDR is IPv4 or IPv6
func DetectIPVersion(cidr string) (IPVersion, error) {
    _, ipNet, err := net.ParseCIDR(cidr)
    if err != nil {
        return 0, fmt.Errorf("invalid CIDR: %w", err)
    }
    
    if ipNet.IP.To4() != nil {
        return IPVersionIPv4, nil
    }
    return IPVersionIPv6, nil
}
```

### 5. DNS Service Extensions

```go
// DNSPeer represents a peer for DNS resolution
type DNSPeer struct {
    Name      string   `json:"name"`
    IPv4      string   `json:"ipv4,omitempty"` // RENAMED from IP
    IPv6      string   `json:"ipv6,omitempty"` // NEW
}

// PeerDNSConfig is sent to jump agents for DNS server startup
type PeerDNSConfig struct {
    IP     string    `json:"ip"`      // Jump peer's IP
    Domain string    `json:"domain"`
    Peers  []DNSPeer `json:"peers"`
}

// DNSService manages DNS records for both IPv4 and IPv6
type DNSService interface {
    CreateDNSMapping(ctx context.Context, networkID, routeID string, req *DNSMappingCreateRequest) (*DNSMapping, error)
    GetDNSMapping(ctx context.Context, networkID, routeID, dnsID string) (*DNSMapping, error)
    UpdateDNSMapping(ctx context.Context, networkID, routeID, dnsID string, req *DNSMappingUpdateRequest) (*DNSMapping, error)
    DeleteDNSMapping(ctx context.Context, networkID, routeID, dnsID string) error
    ListDNSMappings(ctx context.Context, networkID, routeID string) ([]*DNSMapping, error)
    
    // Get all DNS records for a network (A and AAAA)
    GetNetworkDNSRecords(ctx context.Context, networkID string) ([]DNSPeer, error)
}
```

### 6. Firewall Adapter Extensions

```go
// Adapter implements dynamic filtering using iptables and ip6tables
type Adapter struct {
    iface        string
    natInterface string
    httpPort     int
    httpsPort    int
}

// Sync applies forwarding/NAT plus policy-based firewall rules
func (a *Adapter) Sync(p *JumpPolicy, selfIP string, whitelistedIPs []string) error {
    // Enable IPv4 forwarding if needed
    if p.HasIPv4Traffic() {
        exec.Command("sysctl", "-w", "net.ipv4.ip_forward=1").Run()
    }
    
    // Enable IPv6 forwarding if needed
    if p.HasIPv6Traffic() {
        exec.Command("sysctl", "-w", "net.ipv6.conf.all.forwarding=1").Run()
    }
    
    // Apply IPv4 rules
    if len(p.IPTablesRules) > 0 {
        a.applyIPv4Rules(p.IPTablesRules)
    }
    
    // Apply IPv6 rules
    if len(p.IP6TablesRules) > 0 {
        a.applyIPv6Rules(p.IP6TablesRules)
    }
    
    return nil
}

// applyIPv4Rules applies iptables rules
func (a *Adapter) applyIPv4Rules(rules []string) error {
    chain := "WIRETY_JUMP"
    a.run("-N", chain)
    a.run("-F", chain)
    
    for _, rule := range rules {
        a.applyIPTablesRule(chain, rule)
    }
    
    a.runIfNotExists("-I", "FORWARD", "1", "-j", chain)
    return nil
}

// applyIPv6Rules applies ip6tables rules
func (a *Adapter) applyIPv6Rules(rules []string) error {
    chain := "WIRETY_JUMP"
    a.runIPv6("-N", chain)
    a.runIPv6("-F", chain)
    
    for _, rule := range rules {
        a.applyIP6TablesRule(chain, rule)
    }
    
    a.runIPv6IfNotExists("-I", "FORWARD", "1", "-j", chain)
    return nil
}
```

## Data Models

### Database Schema Changes

```sql
-- Add IP stack mode and IPv6 CIDR to networks table
ALTER TABLE networks 
ADD COLUMN ip_stack_mode VARCHAR(10) DEFAULT 'ipv4' NOT NULL,
ADD COLUMN ipv6_cidr VARCHAR(50);

-- Rename address column and add IPv6 address to peers table
ALTER TABLE peers 
RENAME COLUMN address TO ipv4_address;

ALTER TABLE peers
ADD COLUMN ipv6_address VARCHAR(50);

-- Add index for IPv6 addresses
CREATE INDEX idx_peers_ipv6_address ON peers(ipv6_address);

-- Add constraint to ensure at least one IP address exists
ALTER TABLE peers
ADD CONSTRAINT chk_peer_has_ip CHECK (
    ipv4_address IS NOT NULL OR ipv6_address IS NOT NULL
);
```

### IPAM IPv6 Support

The IPAM system will be extended to support IPv6 prefix management:

```go
// IPv6Prefix represents an IPv6 address range
type IPv6Prefix struct {
    CIDR      string
    Available uint128  // Number of available addresses
    Allocated map[string]bool  // Allocated IPv6 addresses
}

// IPv6 address allocation strategy:
// - Use SLAAC-style addressing for simplicity
// - Allocate sequentially from the prefix
// - Track allocated addresses in a bitmap or map
// - Support /64 prefixes (standard for IPv6 networks)
```

## Correctness Properties

*A property is a characteristic or behavior that should hold true across all valid executions of a system-essentially, a formal statement about what the system should do. Properties serve as the bridge between human-readable specifications and machine-verifiable correctness guarantees.*

### Property 1: IP Stack Mode Consistency
*For any* network, the allocated IP addresses for all peers must match the network's IP stack mode (IPv4-only peers have only IPv4, IPv6-only have only IPv6, dual-stack have both).
**Validates: Requirements 1.2, 1.3, 1.4, 3.1, 3.2, 3.3**

### Property 2: CIDR Validation
*For any* network creation or update request, the system must reject configurations where the provided CIDRs don't match the IP stack mode requirements.
**Validates: Requirements 1.2, 1.3, 1.4, 2.1, 2.2**

### Property 3: IP Address Uniqueness
*For any* two peers in the same network, their IPv4 addresses (if present) must be unique, and their IPv6 addresses (if present) must be unique.
**Validates: Requirements 3.1, 3.2, 3.3**

### Property 4: WireGuard Config IP Version Matching
*For any* generated WireGuard configuration, the Interface and AllowedIPs sections must only contain IP addresses matching the network's IP stack mode.
**Validates: Requirements 4.1, 4.2, 4.3, 4.4**

### Property 5: DNS Record Type Matching
*For any* peer with an IPv4 address, an A record must exist; for any peer with an IPv6 address, an AAAA record must exist.
**Validates: Requirements 5.1, 5.2, 5.3**

### Property 6: Firewall Rule IP Version Matching
*For any* policy rule with an IPv4 CIDR target, only iptables rules must be generated; for IPv6 CIDR targets, only ip6tables rules must be generated.
**Validates: Requirements 6.1, 6.2, 6.3, 6.4, 6.5**

### Property 7: Route CIDR IP Version Compatibility
*For any* route, the destination CIDR IP version must be supported by the network's IP stack mode.
**Validates: Requirements 7.1, 7.2, 7.3, 7.5**

### Property 8: DNS Mapping IP Version Validation
*For any* DNS mapping, the IP address must be within the associated route's destination CIDR and match a valid IP version.
**Validates: Requirements 8.1, 8.2, 8.3, 8.4**

### Property 9: Kernel Forwarding Configuration
*For any* jump peer, IPv4 forwarding must be enabled if the network has IPv4, and IPv6 forwarding must be enabled if the network has IPv6.
**Validates: Requirements 9.1, 9.2, 9.3**

### Property 10: IP Stack Mode Migration Consistency
*For any* network IP stack mode change, all peers must receive appropriate IP addresses for the new mode, and old addresses must be released.
**Validates: Requirements 10.1, 10.2, 10.3, 10.4, 10.5**

### Property 11: IPAM Address Allocation Correctness
*For any* IP address allocated by IPAM, it must be within the specified CIDR range and not previously allocated.
**Validates: Requirements 11.1, 11.2, 11.3**

### Property 12: Backward Compatibility
*For any* existing network without an explicit IP stack mode, the system must default to IPv4-only mode and continue functioning correctly.
**Validates: Requirements 14.1, 14.2, 14.3, 14.4, 14.5**

## Error Handling

### IP Stack Mode Validation Errors
- Invalid IP stack mode value → Return 400 Bad Request with clear error message
- Missing required CIDR for mode → Return 400 Bad Request specifying which CIDR is required
- CIDR provided for wrong mode → Return 400 Bad Request explaining the conflict

### IP Address Allocation Errors
- IPv4 CIDR exhausted → Return 507 Insufficient Storage with message about available addresses
- IPv6 CIDR exhausted → Return 507 Insufficient Storage with message about available addresses
- Invalid CIDR format → Return 400 Bad Request with CIDR validation error

### IP Version Mismatch Errors
- Policy rule CIDR doesn't match network mode → Return 400 Bad Request explaining incompatibility
- Route CIDR doesn't match network mode → Return 400 Bad Request explaining incompatibility
- DNS mapping IP doesn't match route CIDR → Return 400 Bad Request with validation details

### Migration Errors
- Cannot change mode with static peers → Return 409 Conflict explaining which peers block the change
- Insufficient addresses for migration → Return 507 Insufficient Storage with details

## Testing Strategy

### Unit Tests
- IP stack mode validation logic
- CIDR parsing and validation for IPv4 and IPv6
- IP address allocation from IPv6 prefixes
- IP version detection from CIDR notation
- WireGuard config generation for each mode
- iptables and ip6tables rule generation

### Property-Based Tests
- Property 1: IP Stack Mode Consistency - Generate random networks and peers, verify IP allocation matches mode
- Property 2: CIDR Validation - Generate random network requests, verify validation logic
- Property 3: IP Address Uniqueness - Generate random peer allocations, verify no duplicates
- Property 4: WireGuard Config Matching - Generate configs, verify IP versions match mode
- Property 5: DNS Record Matching - Generate peers, verify A/AAAA records match IPs
- Property 6: Firewall Rule Matching - Generate policies, verify iptables/ip6tables match CIDR version
- Property 7: Route CIDR Compatibility - Generate routes, verify CIDR versions are compatible
- Property 8: DNS Mapping Validation - Generate DNS mappings, verify IPs are within route CIDRs
- Property 9: Kernel Forwarding - Generate jump peer configs, verify forwarding settings
- Property 10: Migration Consistency - Generate mode changes, verify IP allocation/release
- Property 11: IPAM Correctness - Generate allocations, verify addresses are valid and unique
- Property 12: Backward Compatibility - Load existing networks, verify IPv4-only default

### Integration Tests
- Create IPv4-only network and verify end-to-end connectivity
- Create IPv6-only network and verify end-to-end connectivity
- Create dual-stack network and verify both IPv4 and IPv6 connectivity
- Migrate network from IPv4-only to dual-stack
- Apply policies to dual-stack network and verify both iptables and ip6tables rules
- Test DNS resolution for both A and AAAA records

### Manual Testing Scenarios
- Create network with each IP stack mode via UI
- Add peers and verify correct IP allocation
- Generate WireGuard configs and test connectivity
- Apply policies and verify firewall rules on jump peer
- Query DNS for both A and AAAA records
- Migrate existing IPv4 network to dual-stack
