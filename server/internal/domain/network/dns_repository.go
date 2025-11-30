package network

import "context"

// DNSRepository defines the interface for DNS mapping data persistence
type DNSRepository interface {
	// DNS mapping CRUD operations
	CreateDNSMapping(ctx context.Context, routeID string, mapping *DNSMapping) error
	GetDNSMapping(ctx context.Context, routeID, mappingID string) (*DNSMapping, error)
	UpdateDNSMapping(ctx context.Context, routeID string, mapping *DNSMapping) error
	DeleteDNSMapping(ctx context.Context, routeID, mappingID string) error
	ListDNSMappings(ctx context.Context, routeID string) ([]*DNSMapping, error)

	// Get all DNS mappings for a network (for DNS server configuration)
	GetNetworkDNSMappings(ctx context.Context, networkID string) ([]*DNSMapping, error)
}
