package api

import (
	"context"

	"wirety/internal/application/dns"
	"wirety/internal/domain/network"
)

// DNSServiceAdapter adapts the application DNS service to the API interface
type DNSServiceAdapter struct {
	service *dns.Service
}

// NewDNSServiceAdapter creates a new DNS service adapter
func NewDNSServiceAdapter(service *dns.Service) *DNSServiceAdapter {
	return &DNSServiceAdapter{service: service}
}

// CreateDNSMapping creates a new DNS mapping
func (a *DNSServiceAdapter) CreateDNSMapping(ctx context.Context, routeID string, req *network.DNSMappingCreateRequest) (*network.DNSMapping, error) {
	return a.service.CreateDNSMapping(ctx, routeID, req)
}

// GetDNSMapping retrieves a DNS mapping by ID
func (a *DNSServiceAdapter) GetDNSMapping(ctx context.Context, routeID, mappingID string) (*network.DNSMapping, error) {
	return a.service.GetDNSMapping(ctx, routeID, mappingID)
}

// UpdateDNSMapping updates an existing DNS mapping
func (a *DNSServiceAdapter) UpdateDNSMapping(ctx context.Context, routeID, mappingID string, req *network.DNSMappingUpdateRequest) (*network.DNSMapping, error) {
	return a.service.UpdateDNSMapping(ctx, routeID, mappingID, req)
}

// DeleteDNSMapping deletes a DNS mapping
func (a *DNSServiceAdapter) DeleteDNSMapping(ctx context.Context, routeID, mappingID string) error {
	return a.service.DeleteDNSMapping(ctx, routeID, mappingID)
}

// ListDNSMappings lists all DNS mappings for a route
func (a *DNSServiceAdapter) ListDNSMappings(ctx context.Context, routeID string) ([]*network.DNSMapping, error) {
	return a.service.ListDNSMappings(ctx, routeID)
}

// GetNetworkDNSRecords combines peer and route DNS records
func (a *DNSServiceAdapter) GetNetworkDNSRecords(ctx context.Context, networkID string) ([]DNSRecord, error) {
	records, err := a.service.GetNetworkDNSRecords(ctx, networkID)
	if err != nil {
		return nil, err
	}

	// Convert dns.DNSRecord to api.DNSRecord
	apiRecords := make([]DNSRecord, len(records))
	for i, record := range records {
		apiRecords[i] = DNSRecord{
			Name:      record.Name,
			IPAddress: record.IPAddress,
			FQDN:      record.FQDN,
			Type:      record.Type,
		}
	}

	return apiRecords, nil
}
