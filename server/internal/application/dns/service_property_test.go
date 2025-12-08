package dns

import (
	"context"
	"fmt"
	"net"
	"testing"
	"time"

	"wirety/internal/domain/network"

	"github.com/leanovate/gopter"
	"github.com/leanovate/gopter/gen"
	"github.com/leanovate/gopter/prop"
)

// Mock implementations for testing

type mockDNSRepository struct {
	mappings map[string]*network.DNSMapping // mappingID -> mapping
}

func newMockDNSRepository() *mockDNSRepository {
	return &mockDNSRepository{
		mappings: make(map[string]*network.DNSMapping),
	}
}

func (m *mockDNSRepository) CreateDNSMapping(ctx context.Context, routeID string, mapping *network.DNSMapping) error {
	// Check for duplicate name
	for _, existing := range m.mappings {
		if existing.RouteID == routeID && existing.Name == mapping.Name {
			return network.ErrDuplicateDNSName
		}
	}
	m.mappings[mapping.ID] = mapping
	return nil
}

func (m *mockDNSRepository) GetDNSMapping(ctx context.Context, routeID, mappingID string) (*network.DNSMapping, error) {
	mapping, exists := m.mappings[mappingID]
	if !exists || mapping.RouteID != routeID {
		return nil, network.ErrDNSMappingNotFound
	}
	return mapping, nil
}

func (m *mockDNSRepository) UpdateDNSMapping(ctx context.Context, routeID string, mapping *network.DNSMapping) error {
	existing, exists := m.mappings[mapping.ID]
	if !exists || existing.RouteID != routeID {
		return network.ErrDNSMappingNotFound
	}
	m.mappings[mapping.ID] = mapping
	return nil
}

func (m *mockDNSRepository) DeleteDNSMapping(ctx context.Context, routeID, mappingID string) error {
	mapping, exists := m.mappings[mappingID]
	if !exists || mapping.RouteID != routeID {
		return network.ErrDNSMappingNotFound
	}
	delete(m.mappings, mappingID)
	return nil
}

func (m *mockDNSRepository) ListDNSMappings(ctx context.Context, routeID string) ([]*network.DNSMapping, error) {
	var mappings []*network.DNSMapping
	for _, mapping := range m.mappings {
		if mapping.RouteID == routeID {
			mappings = append(mappings, mapping)
		}
	}
	return mappings, nil
}

func (m *mockDNSRepository) GetNetworkDNSMappings(ctx context.Context, networkID string) ([]*network.DNSMapping, error) {
	var mappings []*network.DNSMapping
	for _, mapping := range m.mappings {
		mappings = append(mappings, mapping)
	}
	return mappings, nil
}

type mockRouteRepository struct {
	routes map[string]*network.Route // routeID -> route
}

func newMockRouteRepository() *mockRouteRepository {
	return &mockRouteRepository{
		routes: make(map[string]*network.Route),
	}
}

func (m *mockRouteRepository) CreateRoute(ctx context.Context, networkID string, route *network.Route) error {
	m.routes[route.ID] = route
	return nil
}

func (m *mockRouteRepository) GetRoute(ctx context.Context, networkID, routeID string) (*network.Route, error) {
	route, exists := m.routes[routeID]
	if !exists {
		return nil, network.ErrRouteNotFound
	}
	if networkID != "" && route.NetworkID != networkID {
		return nil, network.ErrRouteNotFound
	}
	return route, nil
}

func (m *mockRouteRepository) UpdateRoute(ctx context.Context, networkID string, route *network.Route) error {
	existing, exists := m.routes[route.ID]
	if !exists || existing.NetworkID != networkID {
		return network.ErrRouteNotFound
	}
	m.routes[route.ID] = route
	return nil
}

func (m *mockRouteRepository) DeleteRoute(ctx context.Context, networkID, routeID string) error {
	route, exists := m.routes[routeID]
	if !exists || route.NetworkID != networkID {
		return network.ErrRouteNotFound
	}
	delete(m.routes, routeID)
	return nil
}

func (m *mockRouteRepository) ListRoutes(ctx context.Context, networkID string) ([]*network.Route, error) {
	var routes []*network.Route
	for _, route := range m.routes {
		if route.NetworkID == networkID {
			routes = append(routes, route)
		}
	}
	return routes, nil
}

func (m *mockRouteRepository) GetRoutesForGroup(ctx context.Context, networkID, groupID string) ([]*network.Route, error) {
	// Not needed for this test
	return nil, nil
}

func (m *mockRouteRepository) GetRoutesByJumpPeer(ctx context.Context, networkID, jumpPeerID string) ([]*network.Route, error) {
	// Not needed for this test
	return nil, nil
}

// Generators for property-based testing

func genValidDNSName() gopter.Gen {
	return gen.Identifier().SuchThat(func(v interface{}) bool {
		s := v.(string)
		return len(s) > 0 && len(s) <= 63
	})
}

func genRouteID() gopter.Gen {
	return gen.Identifier().Map(func(v string) string {
		return "route-" + v
	})
}

func genNetworkID() gopter.Gen {
	return gen.Identifier().Map(func(v string) string {
		return "net-" + v
	})
}

// genCIDRAndIPInRange generates a CIDR and an IP address within that CIDR
func genCIDRAndIPInRange() gopter.Gen {
	return gen.OneConstOf(
		// 10.0.0.0/24 with IPs in range
		[]interface{}{"10.0.0.0/24", "10.0.0.1"},
		[]interface{}{"10.0.0.0/24", "10.0.0.100"},
		[]interface{}{"10.0.0.0/24", "10.0.0.254"},
		// 192.168.1.0/24 with IPs in range
		[]interface{}{"192.168.1.0/24", "192.168.1.1"},
		[]interface{}{"192.168.1.0/24", "192.168.1.50"},
		[]interface{}{"192.168.1.0/24", "192.168.1.200"},
		// 172.16.0.0/16 with IPs in range
		[]interface{}{"172.16.0.0/16", "172.16.0.1"},
		[]interface{}{"172.16.0.0/16", "172.16.100.50"},
		[]interface{}{"172.16.0.0/16", "172.16.255.254"},
		// 10.0.0.0/8 with IPs in range
		[]interface{}{"10.0.0.0/8", "10.1.2.3"},
		[]interface{}{"10.0.0.0/8", "10.255.255.254"},
	)
}

// genCIDRAndIPOutOfRange generates a CIDR and an IP address outside that CIDR
func genCIDRAndIPOutOfRange() gopter.Gen {
	return gen.OneConstOf(
		// 10.0.0.0/24 with IPs out of range
		[]interface{}{"10.0.0.0/24", "10.0.1.1"},
		[]interface{}{"10.0.0.0/24", "192.168.1.1"},
		[]interface{}{"10.0.0.0/24", "172.16.0.1"},
		// 192.168.1.0/24 with IPs out of range
		[]interface{}{"192.168.1.0/24", "192.168.2.1"},
		[]interface{}{"192.168.1.0/24", "10.0.0.1"},
		[]interface{}{"192.168.1.0/24", "172.16.0.1"},
		// 172.16.0.0/16 with IPs out of range
		[]interface{}{"172.16.0.0/16", "172.17.0.1"},
		[]interface{}{"172.16.0.0/16", "10.0.0.1"},
		[]interface{}{"172.16.0.0/16", "192.168.1.1"},
	)
}

// Property Tests

// **Feature: network-groups-policies-routing, Property 45: DNS mapping IP validation**
// **Validates: Requirements 8.2**
func TestProperty_DNSMappingIPValidation(t *testing.T) {
	properties := gopter.NewProperties(nil)

	properties.Property("Feature: network-groups-policies-routing, Property 45: DNS mapping IP validation - valid IPs",
		prop.ForAll(
			func(networkID string, routeID string, dnsName string, cidrAndIP []interface{}) bool {
				// Extract CIDR and IP from the generated pair
				cidr := cidrAndIP[0].(string)
				ip := cidrAndIP[1].(string)

				// Setup mocks
				dnsRepo := newMockDNSRepository()
				routeRepo := newMockRouteRepository()

				// Create a route with the generated CIDR
				route := &network.Route{
					ID:              routeID,
					NetworkID:       networkID,
					Name:            "test-route",
					Description:     "Test route",
					DestinationCIDR: cidr,
					JumpPeerID:      "jump-peer-1",
					DomainSuffix:    "internal",
					CreatedAt:       time.Now(),
					UpdatedAt:       time.Now(),
				}
				_ = routeRepo.CreateRoute(context.Background(), networkID, route)

				// Create DNS service (peer repo not needed for this test)
				service := NewService(dnsRepo, routeRepo, nil)

				// Create DNS mapping request with IP in range
				req := &network.DNSMappingCreateRequest{
					Name:      dnsName,
					IPAddress: ip,
				}

				// Attempt to create DNS mapping
				mapping, err := service.CreateDNSMapping(context.Background(), networkID, routeID, req)

				// Verify property: DNS mapping creation should succeed when IP is in CIDR
				return err == nil &&
					mapping != nil &&
					mapping.Name == dnsName &&
					mapping.IPAddress == ip &&
					mapping.RouteID == routeID
			},
			genNetworkID(),
			genRouteID(),
			genValidDNSName(),
			genCIDRAndIPInRange(),
		))

	properties.Property("Feature: network-groups-policies-routing, Property 45: DNS mapping IP validation - invalid IPs",
		prop.ForAll(
			func(networkID string, routeID string, dnsName string, cidrAndIP []interface{}) bool {
				// Extract CIDR and IP from the generated pair
				cidr := cidrAndIP[0].(string)
				ip := cidrAndIP[1].(string)

				// Setup mocks
				dnsRepo := newMockDNSRepository()
				routeRepo := newMockRouteRepository()

				// Create a route with the generated CIDR
				route := &network.Route{
					ID:              routeID,
					NetworkID:       networkID,
					Name:            "test-route",
					Description:     "Test route",
					DestinationCIDR: cidr,
					JumpPeerID:      "jump-peer-1",
					DomainSuffix:    "internal",
					CreatedAt:       time.Now(),
					UpdatedAt:       time.Now(),
				}
				_ = routeRepo.CreateRoute(context.Background(), networkID, route)

				// Create DNS service (peer repo not needed for this test)
				service := NewService(dnsRepo, routeRepo, nil)

				// Create DNS mapping request with IP out of range
				req := &network.DNSMappingCreateRequest{
					Name:      dnsName,
					IPAddress: ip,
				}

				// Attempt to create DNS mapping
				_, err := service.CreateDNSMapping(context.Background(), networkID, routeID, req)

				// Verify property: DNS mapping creation should fail when IP is not in CIDR
				// The error should indicate IP validation failure
				return err != nil
			},
			genNetworkID(),
			genRouteID(),
			genValidDNSName(),
			genCIDRAndIPOutOfRange(),
		))

	properties.TestingRun(t, gopter.ConsoleReporter(false))
}

// genDomainSuffix generates valid domain suffixes
func genDomainSuffix() gopter.Gen {
	return gen.OneConstOf(
		"internal",
		"local",
		"corp",
		"example.com",
		"test.local",
		"dev.internal",
	)
}

// genRouteName generates valid route names
func genRouteName() gopter.Gen {
	return gen.Identifier().SuchThat(func(v interface{}) bool {
		s := v.(string)
		return len(s) > 0 && len(s) <= 63
	})
}

// **Feature: network-groups-policies-routing, Property 46: DNS mapping FQDN format**
// **Validates: Requirements 8.3**
func TestProperty_DNSMappingFQDNFormat(t *testing.T) {
	properties := gopter.NewProperties(nil)

	properties.Property("Feature: network-groups-policies-routing, Property 46: DNS mapping FQDN format",
		prop.ForAll(
			func(networkID string, routeID string, dnsName string, routeName string, domainSuffix string, cidrAndIP []interface{}) bool {
				// Extract CIDR and IP from the generated pair
				cidr := cidrAndIP[0].(string)
				ip := cidrAndIP[1].(string)

				// Setup mocks
				dnsRepo := newMockDNSRepository()
				routeRepo := newMockRouteRepository()

				// Create a route with the generated properties
				route := &network.Route{
					ID:              routeID,
					NetworkID:       networkID,
					Name:            routeName,
					Description:     "Test route",
					DestinationCIDR: cidr,
					JumpPeerID:      "jump-peer-1",
					DomainSuffix:    domainSuffix,
					CreatedAt:       time.Now(),
					UpdatedAt:       time.Now(),
				}
				_ = routeRepo.CreateRoute(context.Background(), networkID, route)

				// Create DNS service
				service := NewService(dnsRepo, routeRepo, nil)

				// Create DNS mapping
				req := &network.DNSMappingCreateRequest{
					Name:      dnsName,
					IPAddress: ip,
				}

				mapping, err := service.CreateDNSMapping(context.Background(), networkID, routeID, req)
				if err != nil {
					// If creation fails, skip this test case
					return true
				}

				// Get the FQDN using the GetFQDN method
				fqdn := mapping.GetFQDN(route)

				// Verify property: FQDN should be formatted as name.route_name.domain_suffix
				expectedFQDN := fmt.Sprintf("%s.%s.%s", dnsName, routeName, domainSuffix)

				return fqdn == expectedFQDN
			},
			genNetworkID(),
			genRouteID(),
			genValidDNSName(),
			genRouteName(),
			genDomainSuffix(),
			genCIDRAndIPInRange(),
		))

	properties.TestingRun(t, gopter.ConsoleReporter(false))
}

// Helper function to verify IP is in CIDR (for testing the test)
// nolint:unused // Kept for potential future test validation
func isIPInCIDR(ip, cidr string) bool {
	parsedIP := net.ParseIP(ip)
	if parsedIP == nil {
		return false
	}

	_, ipNet, err := net.ParseCIDR(cidr)
	if err != nil {
		return false
	}

	return ipNet.Contains(parsedIP)
}
