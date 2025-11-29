# Requirements Document

## Introduction

This specification defines the requirements for adding flexible IP stack support to the Wirety WireGuard network management system. The system currently only supports IPv4 networks. This feature will enable users to create networks using IPv4-only, IPv6-only, or dual-stack (IPv4 + IPv6) configurations based on their specific needs.

## Glossary

- **System**: The Wirety WireGuard network management system
- **Network**: A WireGuard virtual private network managed by the System
- **Peer**: A device or endpoint connected to a Network
- **Jump Peer**: A peer that routes traffic for other peers and enforces policies
- **IP Stack Mode**: The IP protocol configuration for a Network (ipv4, ipv6, or dual)
- **CIDR**: Classless Inter-Domain Routing notation for IP address ranges
- **ULA**: Unique Local Address - IPv6 private address space (fc00::/7)
- **IPAM**: IP Address Management system
- **Dual-Stack**: A network configuration supporting both IPv4 and IPv6 simultaneously
- **iptables**: Linux firewall for IPv4 packet filtering
- **ip6tables**: Linux firewall for IPv6 packet filtering

## Requirements

### Requirement 1

**User Story:** As a network administrator, I want to choose the IP stack mode when creating a network, so that I can use IPv4-only, IPv6-only, or dual-stack based on my infrastructure needs.

#### Acceptance Criteria

1. WHEN creating a network THEN the System SHALL accept an ip_stack_mode parameter with values "ipv4", "ipv6", or "dual"
2. WHEN ip_stack_mode is "ipv4" THEN the System SHALL require an IPv4 CIDR and SHALL NOT require an IPv6 CIDR
3. WHEN ip_stack_mode is "ipv6" THEN the System SHALL require an IPv6 CIDR and SHALL NOT require an IPv4 CIDR
4. WHEN ip_stack_mode is "dual" THEN the System SHALL require both an IPv4 CIDR and an IPv6 CIDR
5. WHEN ip_stack_mode is not provided THEN the System SHALL default to "ipv4" for backward compatibility

### Requirement 2

**User Story:** As a network administrator, I want the system to validate IP addresses according to the network's IP stack mode, so that configuration errors are caught early.

#### Acceptance Criteria

1. WHEN validating an IPv4 CIDR THEN the System SHALL verify it follows IPv4 notation (e.g., 10.0.0.0/24)
2. WHEN validating an IPv6 CIDR THEN the System SHALL verify it follows IPv6 notation (e.g., fd00:1234::/64)
3. WHEN a network uses IPv4-only mode THEN the System SHALL reject IPv6 addresses
4. WHEN a network uses IPv6-only mode THEN the System SHALL reject IPv4 addresses
5. WHEN a network uses dual-stack mode THEN the System SHALL accept both IPv4 and IPv6 addresses

### Requirement 3

**User Story:** As a network administrator, I want peers to receive IP addresses matching the network's IP stack mode, so that connectivity works correctly.

#### Acceptance Criteria

1. WHEN adding a peer to an IPv4-only network THEN the System SHALL allocate one IPv4 address from the network CIDR
2. WHEN adding a peer to an IPv6-only network THEN the System SHALL allocate one IPv6 address from the network CIDR
3. WHEN adding a peer to a dual-stack network THEN the System SHALL allocate both an IPv4 address and an IPv6 address
4. WHEN allocating IPv6 addresses THEN the System SHALL use SLAAC-compatible addressing or DHCPv6-style allocation
5. WHEN a peer is deleted THEN the System SHALL release all allocated IP addresses back to the IPAM pool

### Requirement 4

**User Story:** As a network administrator, I want WireGuard configurations to include the correct IP addresses and AllowedIPs based on the IP stack mode, so that peers can communicate properly.

#### Acceptance Criteria

1. WHEN generating WireGuard config for an IPv4-only peer THEN the System SHALL include only IPv4 addresses in the Interface and Peer sections
2. WHEN generating WireGuard config for an IPv6-only peer THEN the System SHALL include only IPv6 addresses in the Interface and Peer sections
3. WHEN generating WireGuard config for a dual-stack peer THEN the System SHALL include both IPv4 and IPv6 addresses in the Interface section
4. WHEN generating AllowedIPs for peer-to-peer communication THEN the System SHALL include the network CIDR(s) matching the IP stack mode
5. WHEN generating AllowedIPs for routes THEN the System SHALL include route CIDRs matching their IP version

### Requirement 5

**User Story:** As a network administrator, I want DNS records to support both A and AAAA records based on peer IP addresses, so that name resolution works for all IP versions.

#### Acceptance Criteria

1. WHEN a peer has an IPv4 address THEN the System SHALL create an A record for the peer's hostname
2. WHEN a peer has an IPv6 address THEN the System SHALL create an AAAA record for the peer's hostname
3. WHEN a peer has both IPv4 and IPv6 addresses THEN the System SHALL create both A and AAAA records
4. WHEN querying DNS for a hostname THEN the DNS server SHALL return records matching the query type (A or AAAA)
5. WHEN a peer is deleted THEN the System SHALL remove all associated DNS records

### Requirement 6

**User Story:** As a network administrator, I want firewall policies to apply to both IPv4 and IPv6 traffic based on the network's IP stack mode, so that security rules are enforced consistently.

#### Acceptance Criteria

1. WHEN a network uses IPv4-only mode THEN the System SHALL generate only iptables rules
2. WHEN a network uses IPv6-only mode THEN the System SHALL generate only ip6tables rules
3. WHEN a network uses dual-stack mode THEN the System SHALL generate both iptables and ip6tables rules
4. WHEN a policy rule specifies an IPv4 CIDR target THEN the System SHALL generate iptables rules
5. WHEN a policy rule specifies an IPv6 CIDR target THEN the System SHALL generate ip6tables rules
6. WHEN a policy rule specifies 0.0.0.0/0 THEN the System SHALL apply it to all IPv4 traffic
7. WHEN a policy rule specifies ::/0 THEN the System SHALL apply it to all IPv6 traffic

### Requirement 7

**User Story:** As a network administrator, I want to create routes with IPv4 or IPv6 destination CIDRs, so that I can route traffic to external networks regardless of IP version.

#### Acceptance Criteria

1. WHEN creating a route THEN the System SHALL accept either an IPv4 or IPv6 destination CIDR
2. WHEN a route has an IPv4 destination CIDR THEN the System SHALL only apply it to peers with IPv4 addresses
3. WHEN a route has an IPv6 destination CIDR THEN the System SHALL only apply it to peers with IPv6 addresses
4. WHEN a network is dual-stack THEN the System SHALL allow routes with both IPv4 and IPv6 destination CIDRs
5. WHEN validating a route THEN the System SHALL verify the destination CIDR matches a supported IP version for the network

### Requirement 8

**User Story:** As a network administrator, I want DNS mappings to support both IPv4 and IPv6 addresses, so that I can map hostnames to any IP version.

#### Acceptance Criteria

1. WHEN creating a DNS mapping THEN the System SHALL accept either an IPv4 or IPv6 IP address
2. WHEN a DNS mapping has an IPv4 address THEN the System SHALL create an A record
3. WHEN a DNS mapping has an IPv6 address THEN the System SHALL create an AAAA record
4. WHEN validating a DNS mapping IP THEN the System SHALL verify it falls within the associated route's destination CIDR
5. WHEN a route is deleted THEN the System SHALL delete all associated DNS mappings

### Requirement 9

**User Story:** As a network administrator, I want the jump peer to enable IPv4 and/or IPv6 forwarding based on the network's IP stack mode, so that routing works correctly.

#### Acceptance Criteria

1. WHEN a jump peer joins an IPv4-only network THEN the System SHALL enable net.ipv4.ip_forward=1
2. WHEN a jump peer joins an IPv6-only network THEN the System SHALL enable net.ipv6.conf.all.forwarding=1
3. WHEN a jump peer joins a dual-stack network THEN the System SHALL enable both IPv4 and IPv6 forwarding
4. WHEN a jump peer applies firewall rules THEN the System SHALL apply iptables for IPv4 and ip6tables for IPv6
5. WHEN a jump peer has no IPv6 traffic THEN the System SHALL NOT generate ip6tables rules

### Requirement 10

**User Story:** As a network administrator, I want to update an existing network's IP stack mode, so that I can migrate from IPv4-only to dual-stack or vice versa.

#### Acceptance Criteria

1. WHEN updating a network to add IPv6 THEN the System SHALL allocate IPv6 addresses to all existing peers
2. WHEN updating a network to remove IPv6 THEN the System SHALL release all IPv6 addresses and remove AAAA DNS records
3. WHEN updating a network to add IPv4 THEN the System SHALL allocate IPv4 addresses to all existing peers
4. WHEN updating a network to remove IPv4 THEN the System SHALL release all IPv4 addresses and remove A DNS records
5. WHEN changing IP stack mode THEN the System SHALL regenerate WireGuard configurations for all peers
6. WHEN a network has static (non-agent) peers THEN the System SHALL prevent IP stack mode changes that would break their configuration

### Requirement 11

**User Story:** As a developer, I want the IPAM system to support IPv6 address allocation, so that IPv6 addresses are managed consistently with IPv4.

#### Acceptance Criteria

1. WHEN an IPv6 CIDR is registered THEN the IPAM system SHALL track available addresses within that range
2. WHEN allocating an IPv6 address THEN the IPAM system SHALL return an unused address from the IPv6 CIDR
3. WHEN releasing an IPv6 address THEN the IPAM system SHALL mark it as available for reuse
4. WHEN querying IPAM for available addresses THEN the System SHALL support both IPv4 and IPv6 queries
5. WHEN an IPv6 CIDR is exhausted THEN the IPAM system SHALL return an error indicating no addresses are available

### Requirement 12

**User Story:** As a network administrator, I want the frontend UI to support creating and managing networks with different IP stack modes, so that I can configure networks through the web interface.

#### Acceptance Criteria

1. WHEN creating a network in the UI THEN the System SHALL display an IP stack mode selector with options: IPv4, IPv6, Dual-Stack
2. WHEN IPv4 or Dual-Stack is selected THEN the UI SHALL display an IPv4 CIDR input field
3. WHEN IPv6 or Dual-Stack is selected THEN the UI SHALL display an IPv6 CIDR input field
4. WHEN viewing a network THEN the UI SHALL display the current IP stack mode and associated CIDRs
5. WHEN viewing a peer THEN the UI SHALL display all allocated IP addresses (IPv4 and/or IPv6)

### Requirement 13

**User Story:** As a network administrator, I want policy rules to automatically apply to the correct IP version, so that I don't need to create duplicate rules for IPv4 and IPv6.

#### Acceptance Criteria

1. WHEN a policy rule targets a CIDR THEN the System SHALL detect whether it is IPv4 or IPv6 based on notation
2. WHEN a policy rule targets 0.0.0.0/0 THEN the System SHALL apply it only to IPv4 traffic
3. WHEN a policy rule targets ::/0 THEN the System SHALL apply it only to IPv6 traffic
4. WHEN a network is dual-stack and a rule targets "any" THEN the System SHALL generate both iptables and ip6tables rules
5. WHEN displaying policy rules in the UI THEN the System SHALL indicate which IP version each rule applies to

### Requirement 14

**User Story:** As a network administrator, I want backward compatibility with existing IPv4-only networks, so that upgrading the system doesn't break existing configurations.

#### Acceptance Criteria

1. WHEN loading an existing network without ip_stack_mode THEN the System SHALL default to "ipv4" mode
2. WHEN an existing network has only an IPv4 CIDR THEN the System SHALL operate in IPv4-only mode
3. WHEN generating configs for existing peers THEN the System SHALL continue using IPv4 addresses
4. WHEN existing policies reference IPv4 CIDRs THEN the System SHALL continue generating iptables rules
5. WHEN the system is upgraded THEN all existing networks SHALL continue functioning without manual intervention
