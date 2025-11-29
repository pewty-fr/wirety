# Implementation Plan

- [ ] 1. Database schema and domain model updates
- [ ] 1.1 Create migration for IP stack mode and IPv6 support
  - Add ip_stack_mode column to networks table (default 'ipv4')
  - Add ipv6_cidr column to networks table
  - Rename peers.address to peers.ipv4_address
  - Add ipv6_address column to peers table
  - Add constraint ensuring at least one IP address exists
  - Add indexes for IPv6 addresses
  - _Requirements: 1.1, 1.2, 1.3, 1.4, 3.1, 3.2, 3.3_

- [ ] 1.2 Update Network domain model
  - Add IPStackMode type (ipv4, ipv6, dual)
  - Rename CIDR field to IPv4CIDR
  - Add IPv6CIDR field
  - Add IPStackMode field with default "ipv4"
  - Update NetworkCreateRequest with IP stack mode and dual CIDRs
  - Update NetworkUpdateRequest to support IP stack mode changes
  - _Requirements: 1.1, 1.2, 1.3, 1.4, 1.5_

- [ ] 1.3 Update Peer domain model
  - Rename Address field to IPv4Address
  - Add IPv6Address field
  - Add GetAddresses() method returning all IPs
  - Add HasIPv4() and HasIPv6() helper methods
  - Update PeerCreateRequest and PeerUpdateRequest
  - _Requirements: 3.1, 3.2, 3.3_

- [ ] 2. IPAM system IPv6 support
- [ ] 2.1 Extend IPAM repository interface
  - Add AcquireIPv6(ctx, cidr) method
  - Add ReleaseIPv6(ctx, cidr, ip) method
  - Add EnsureIPv6RootPrefix(ctx, cidr) method
  - Add IsIPv6CIDR(cidr) utility method
  - _Requirements: 11.1, 11.2, 11.3, 11.4_

- [ ] 2.2 Implement IPv6 address allocation in PostgreSQL IPAM
  - Implement IPv6 prefix storage and tracking
  - Implement sequential IPv6 address allocation
  - Implement IPv6 address release and reuse
  - Handle /64 and other common IPv6 prefix sizes
  - _Requirements: 11.1, 11.2, 11.3, 11.5_

- [ ] 2.3 Implement IPv6 address allocation in memory IPAM
  - Implement in-memory IPv6 prefix tracking
  - Implement IPv6 address allocation logic
  - Implement IPv6 address release
  - _Requirements: 11.1, 11.2, 11.3_

- [ ] 2.4 Write property test for IPv6 IPAM allocation
  - **Property 11: IPAM address allocation correctness**
  - **Validates: Requirements 11.1, 11.2, 11.3**

- [ ] 3. Network service updates
- [ ] 3.1 Update CreateNetwork for IP stack mode
  - Validate IP stack mode parameter
  - Validate IPv4 CIDR when required by mode
  - Validate IPv6 CIDR when required by mode
  - Ensure IPv6 root prefix in IPAM for IPv6/dual modes
  - Set default IP stack mode to "ipv4" for backward compatibility
  - _Requirements: 1.1, 1.2, 1.3, 1.4, 1.5, 2.1, 2.2_

- [ ] 3.2 Write property test for CIDR validation
  - **Property 2: CIDR validation**
  - **Validates: Requirements 1.2, 1.3, 1.4, 2.1, 2.2**

- [ ] 3.3 Update AddPeer for dual IP allocation
  - Allocate IPv4 address for IPv4-only and dual-stack networks
  - Allocate IPv6 address for IPv6-only and dual-stack networks
  - Store both addresses in peer record
  - Handle allocation failures gracefully
  - _Requirements: 3.1, 3.2, 3.3, 3.4_

- [ ] 3.4 Write property test for IP stack mode consistency
  - **Property 1: IP stack mode consistency**
  - **Validates: Requirements 1.2, 1.3, 1.4, 3.1, 3.2, 3.3**

- [ ] 3.5 Write property test for IP address uniqueness
  - **Property 3: IP address uniqueness**
  - **Validates: Requirements 3.1, 3.2, 3.3**

- [ ] 3.6 Update DeletePeer for dual IP release
  - Release IPv4 address if present
  - Release IPv6 address if present
  - Handle release failures gracefully
  - _Requirements: 3.5_

- [ ] 3.5 Update UpdateNetwork for IP stack mode migration
  - Validate IP stack mode change is allowed
  - Check for static (non-agent) peers that would break
  - Allote new IP addresses when adding IP version
  - Release old IP addresses when removing IP version
  - Regenerate WireGuard configs for all peers
  - Notify all peers via WebSocket
  - _Requirements: 10.1, 10.2, 10.3, 10.4, 10.5, 10.6_

- [ ]* 3.6 Write property test for IP stack mode migration
  - **Property 10: IP stack mode migration consistency**
  - **Validates: Requirements 10.1, 10.2, 10.3, 10.4, 10.5**

- [ ] 4. WireGuard configuration generation
- [ ] 4.1 Update GeneratePeerConfig for multi-IP support
  - Include IPv4 address in Interface section when present
  - Include IPv6 address in Interface section when present
  - Generate AllowedIPs with network CIDRs matching IP stack mode
  - Include route CIDRs matching their IP version
  - Handle IPv4-only, IPv6-only, and dual-stack peers
  - _Requirements: 4.1, 4.2, 4.3, 4.4, 4.5_

- [ ] 4.2 Write property test for WireGuard config IP version matching
  - **Property 4: WireGuard config IP version matching**
  - **Validates: Requirements 4.1, 4.2, 4.3, 4.4**

- [ ] 4.3 Update GeneratePeerConfigWithDNS for multi-IP support
  - Pass both IPv4 and IPv6 addresses to DNS config
  - Generate policy rules for both IP versions
  - Include both IP addresses in peer list for jump peers
  - _Requirements: 4.1, 4.2, 4.3, 5.1, 5.2, 5.3_

- [ ] 5. Policy service IPv6 support
- [ ] 5.1 Add IP version detection utility
  - Implement DetectIPVersion(cidr) function
  - Parse CIDR and determine if IPv4 or IPv6
  - Handle special cases (0.0.0.0/0 vs ::/0)
  - _Requirements: 6.4, 6.5, 6.6, 6.7, 13.1_

- [ ] 5.2 Implement GenerateIP6TablesRules method
  - Copy logic from GenerateIPTablesRules
  - Generate ip6tables commands instead of iptables
  - Handle IPv6 CIDR notation
  - Add DNS rules for IPv6 (UDP port 53)
  - Add default deny rule for IPv6 FORWARD chain
  - _Requirements: 6.2, 6.3, 6.5, 6.7_

- [ ] 5.3 Update GenerateIPTablesRules to filter IPv4 only
  - Detect IP version of each policy rule CIDR
  - Only generate iptables rules for IPv4 CIDRs
  - Skip IPv6 CIDRs (handled by GenerateIP6TablesRules)
  - _Requirements: 6.1, 6.4, 6.6_

- [ ] 5.4 Write property test for firewall rule IP version matching
  - **Property 6: Firewall rule IP version matching**
  - **Validates: Requirements 6.1, 6.2, 6.3, 6.4, 6.5**

- [ ] 5.5 Update policy rule generation in NetworkService
  - Call GenerateIPTablesRules for IPv4 rules
  - Call GenerateIP6TablesRules for IPv6 rules
  - Include both rule sets in JumpPolicy
  - Handle IPv4-only, IPv6-only, and dual-stack networks
  - _Requirements: 6.1, 6.2, 6.3_

- [ ] 6. DNS service IPv6 support
- [ ] 6.1 Update DNSPeer model for dual IPs
  - Rename IP field to IPv4
  - Add IPv6 field
  - Update JSON serialization
  - _Requirements: 5.1, 5.2, 5.3_

- [ ] 6.2 Update DNS record generation in NetworkService
  - Create A records for peers with IPv4 addresses
  - Create AAAA records for peers with IPv6 addresses
  - Include both record types in DNS config for jump peers
  - _Requirements: 5.1, 5.2, 5.3, 5.4_

- [ ] 6.3 Write property test for DNS record type matching
  - **Property 5: DNS record type matching**
  - **Validates: Requirements 5.1, 5.2, 5.3**

- [ ] 6.3 Update DNS mapping validation
  - Detect IP version of DNS mapping IP address
  - Validate IP is within route's destination CIDR
  - Support both IPv4 and IPv6 DNS mappings
  - _Requirements: 8.1, 8.2, 8.3, 8.4_

- [ ] 6.4 Write property test for DNS mapping IP version validation
  - **Property 8: DNS mapping IP version validation**
  - **Validates: Requirements 8.1, 8.2, 8.3, 8.4**

- [ ] 6.5 Update DNS record cleanup on peer deletion
  - Remove A records when IPv4 address is released
  - Remove AAAA records when IPv6 address is released
  - _Requirements: 5.5_

- [ ] 7. Route service IPv6 support
- [ ] 7.1 Update route CIDR validation
  - Detect IP version of destination CIDR
  - Validate CIDR is compatible with network's IP stack mode
  - Allow IPv4 CIDRs for IPv4-only and dual-stack networks
  - Allow IPv6 CIDRs for IPv6-only and dual-stack networks
  - _Requirements: 7.1, 7.2, 7.3, 7.4, 7.5_

- [ ] 7.2 Write property test for route CIDR IP version compatibility
  - **Property 7: Route CIDR IP version compatibility**
  - **Validates: Requirements 7.1, 7.2, 7.3, 7.5**

- [ ] 7.3 Update route application logic
  - Apply IPv4 routes only to peers with IPv4 addresses
  - Apply IPv6 routes only to peers with IPv6 addresses
  - Include appropriate routes in WireGuard AllowedIPs
  - _Requirements: 7.2, 7.3_

- [ ] 8. Agent firewall adapter IPv6 support
- [ ] 8.1 Add ip6tables command execution
  - Implement runIPv6(args) method for ip6tables
  - Implement runIPv6IfNotExists(args) method
  - Add IPv6 rule existence checking
  - _Requirements: 6.2, 6.3, 9.4_

- [ ] 8.2 Implement applyIPv6Rules method
  - Create WIRETY_JUMP chain for ip6tables
  - Flush and populate chain with IPv6 rules
  - Apply rules from JumpPolicy.IP6TablesRules
  - Add default deny rule for IPv6 FORWARD
  - Attach chain to FORWARD
  - _Requirements: 6.2, 6.3, 9.4_

- [ ] 8.3 Update Sync method for dual-stack support
  - Enable IPv4 forwarding when IPv4 rules present
  - Enable IPv6 forwarding when IPv6 rules present
  - Call applyIPv4Rules for iptables
  - Call applyIPv6Rules for ip6tables
  - Handle IPv4-only, IPv6-only, and dual-stack configurations
  - _Requirements: 9.1, 9.2, 9.3, 9.4, 9.5_

- [ ] 8.4 Write property test for kernel forwarding configuration
  - **Property 9: Kernel forwarding configuration**
  - **Validates: Requirements 9.1, 9.2, 9.3**

- [ ] 8.5 Add IPv6 NAT support (optional)
  - Implement IPv6 MASQUERADE for NAT interface
  - Handle systems without IPv6 NAT support gracefully
  - _Requirements: 9.4_

- [ ] 9. Agent DNS server IPv6 support
- [ ] 9.1 Update DNS server to handle AAAA queries
  - Parse AAAA query type
  - Look up IPv6 address for hostname
  - Return AAAA record in response
  - _Requirements: 5.4_

- [ ] 9.2 Update DNS record storage
  - Store both IPv4 and IPv6 addresses for each hostname
  - Support querying by record type (A or AAAA)
  - _Requirements: 5.1, 5.2, 5.3, 5.4_

- [ ] 9.3 Update DNS configuration processing
  - Parse IPv4 and IPv6 fields from DNSPeer
  - Store both address types in DNS server
  - _Requirements: 5.1, 5.2, 5.3_

- [ ] 10. API handlers and validation
- [ ] 10.1 Update network API handlers
  - Accept ip_stack_mode in CreateNetwork request
  - Accept ipv4_cidr and ipv6_cidr fields
  - Validate IP stack mode and CIDRs
  - Return both CIDRs in network responses
  - _Requirements: 1.1, 1.2, 1.3, 1.4, 1.5_

- [ ] 10.2 Update peer API handlers
  - Return both ipv4_address and ipv6_address in responses
  - Handle dual IP addresses in peer listings
  - _Requirements: 3.1, 3.2, 3.3_

- [ ] 10.3 Update policy API handlers
  - Validate policy rule CIDRs against network IP stack mode
  - Display IP version for each rule in responses
  - _Requirements: 6.4, 6.5, 13.5_

- [ ] 10.4 Update route API handlers
  - Validate route destination CIDR against network IP stack mode
  - Display IP version for routes in responses
  - _Requirements: 7.1, 7.5_

- [ ] 10.5 Update DNS mapping API handlers
  - Validate DNS mapping IP address version
  - Support both IPv4 and IPv6 addresses
  - _Requirements: 8.1, 8.2, 8.3_

- [ ] 11. Frontend UI updates
- [ ] 11.1 Update network creation form
  - Add IP stack mode selector (IPv4, IPv6, Dual-Stack)
  - Show/hide IPv4 CIDR field based on mode
  - Show/hide IPv6 CIDR field based on mode
  - Validate CIDRs match selected mode
  - _Requirements: 12.1, 12.2, 12.3_

- [ ] 11.2 Update network detail page
  - Display IP stack mode
  - Display IPv4 CIDR when present
  - Display IPv6 CIDR when present
  - _Requirements: 12.4_

- [ ] 11.3 Update peer list and detail pages
  - Display IPv4 address when present
  - Display IPv6 address when present
  - Show both addresses for dual-stack peers
  - _Requirements: 12.5_

- [ ] 11.4 Update policy rule display
  - Show IP version indicator for each rule (IPv4/IPv6)
  - Auto-detect IP version from CIDR notation
  - _Requirements: 13.5_

- [ ] 11.5 Update route display
  - Show IP version for destination CIDR
  - Filter routes by IP version if needed
  - _Requirements: 7.1_

- [ ] 12. Documentation and migration
- [ ] 12.1 Write IPv6 support documentation
  - Document IP stack mode options
  - Provide examples for each mode
  - Explain IPv6 address allocation
  - Document firewall rule generation for IPv6
  - _Requirements: All requirements_

- [ ] 12.2 Write migration guide for existing deployments
  - Explain backward compatibility (default IPv4-only)
  - Provide steps to migrate to dual-stack
  - Document database migration process
  - Explain impact on existing peers and policies
  - _Requirements: 14.1, 14.2, 14.3, 14.4, 14.5_

- [ ] 12.3 Update API documentation
  - Document new IP stack mode parameter
  - Document IPv4 and IPv6 CIDR fields
  - Document dual IP address responses
  - Update all affected endpoints
  - _Requirements: All API requirements_

- [ ] 13. Property-based tests (comprehensive)
- [ ] 13.1 Write property test for backward compatibility
  - **Property 12: Backward compatibility**
  - **Validates: Requirements 14.1, 14.2, 14.3, 14.4, 14.5**

- [ ] 14. Integration testing
- [ ] 14.1 Test IPv4-only network end-to-end
  - Create IPv4-only network
  - Add peers and verify IPv4 allocation
  - Generate WireGuard configs
  - Apply policies and verify iptables rules
  - Test DNS A record resolution
  - _Requirements: 1.2, 3.1, 4.1, 6.1, 5.1_

- [ ] 14.2 Test IPv6-only network end-to-end
  - Create IPv6-only network
  - Add peers and verify IPv6 allocation
  - Generate WireGuard configs
  - Apply policies and verify ip6tables rules
  - Test DNS AAAA record resolution
  - _Requirements: 1.3, 3.2, 4.2, 6.2, 5.2_

- [ ] 14.3 Test dual-stack network end-to-end
  - Create dual-stack network
  - Add peers and verify dual IP allocation
  - Generate WireGuard configs with both IPs
  - Apply policies and verify both iptables and ip6tables
  - Test DNS A and AAAA record resolution
  - _Requirements: 1.4, 3.3, 4.3, 6.3, 5.3_

- [ ] 14.4 Test IP stack mode migration
  - Create IPv4-only network
  - Migrate to dual-stack
  - Verify IPv6 addresses allocated
  - Verify both iptables and ip6tables rules
  - Test connectivity on both IP versions
  - _Requirements: 10.1, 10.2, 10.3, 10.4, 10.5_

- [ ] 15. Final checkpoint - Ensure all tests pass
  - Ensure all tests pass, ask the user if questions arise.

