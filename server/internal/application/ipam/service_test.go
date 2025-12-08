package ipam

import (
	"context"
	"errors"
	"testing"

	"wirety/internal/domain/network"
)

// mockIPAMRepository implements ipam.Repository for testing
type mockIPAMRepository struct {
	prefixes map[string]*network.IPAMPrefix
	nextIP   map[string]int // CIDR -> next IP counter
}

func newMockIPAMRepository() *mockIPAMRepository {
	return &mockIPAMRepository{
		prefixes: make(map[string]*network.IPAMPrefix),
		nextIP:   make(map[string]int),
	}
}

func (m *mockIPAMRepository) EnsureRootPrefix(ctx context.Context, cidr string) (*network.IPAMPrefix, error) {
	if prefix, exists := m.prefixes[cidr]; exists {
		return prefix, nil
	}

	// Calculate usable hosts for the CIDR
	usableHosts := calculateUsableHosts(cidr)

	prefix := &network.IPAMPrefix{
		CIDR:        cidr,
		ParentCIDR:  "",
		UsableHosts: usableHosts,
	}

	m.prefixes[cidr] = prefix
	return prefix, nil
}

func (m *mockIPAMRepository) AcquireChildPrefix(ctx context.Context, parentCIDR string, prefixLen uint8) (*network.IPAMPrefix, error) {
	// Simple mock implementation - just return a child prefix
	childCIDR := "10.0.1.0/24" // Mock child CIDR

	usableHosts := calculateUsableHosts(childCIDR)

	prefix := &network.IPAMPrefix{
		CIDR:        childCIDR,
		ParentCIDR:  parentCIDR,
		UsableHosts: usableHosts,
	}

	m.prefixes[childCIDR] = prefix
	return prefix, nil
}

func (m *mockIPAMRepository) AcquireSpecificChildPrefix(ctx context.Context, parentCIDR string, cidr string) (*network.IPAMPrefix, error) {
	if _, exists := m.prefixes[cidr]; exists {
		return nil, errors.New("prefix already exists")
	}

	usableHosts := calculateUsableHosts(cidr)

	prefix := &network.IPAMPrefix{
		CIDR:        cidr,
		ParentCIDR:  parentCIDR,
		UsableHosts: usableHosts,
	}

	m.prefixes[cidr] = prefix
	return prefix, nil
}

func (m *mockIPAMRepository) ReleaseChildPrefix(ctx context.Context, cidr string) error {
	delete(m.prefixes, cidr)
	return nil
}

func (m *mockIPAMRepository) DeletePrefix(ctx context.Context, cidr string) error {
	delete(m.prefixes, cidr)
	return nil
}

func (m *mockIPAMRepository) ListChildPrefixes(ctx context.Context, parentCIDR string) ([]*network.IPAMPrefix, error) {
	var children []*network.IPAMPrefix
	for _, prefix := range m.prefixes {
		if prefix.ParentCIDR == parentCIDR {
			children = append(children, prefix)
		}
	}
	return children, nil
}

func (m *mockIPAMRepository) AcquireIP(ctx context.Context, cidr string) (string, error) {
	counter := m.nextIP[cidr]
	m.nextIP[cidr] = counter + 1

	// Simple mock IP allocation
	return "10.0.0.10", nil
}

func (m *mockIPAMRepository) ReleaseIP(ctx context.Context, cidr string, ip string) error {
	return nil
}

// Helper function to calculate usable hosts from CIDR
func calculateUsableHosts(cidr string) int {
	// Simple calculation for /24 networks
	if cidr == "10.0.0.0/24" || cidr == "10.0.1.0/24" {
		return 254 // 256 - 2 (network and broadcast)
	}
	if cidr == "10.0.0.0/16" {
		return 65534 // 65536 - 2
	}
	if cidr == "10.0.0.0/8" {
		return 16777214 // 16777216 - 2
	}
	return 254 // Default for testing
}

func TestNewService(t *testing.T) {
	repo := newMockIPAMRepository()
	service := NewService(repo)

	if service == nil {
		t.Error("Expected service to be created, got nil")
	}

	if service.repo != repo {
		t.Error("Expected service to use provided repository")
	}
}

func TestService_SuggestCIDRs_ValidInput(t *testing.T) {
	repo := newMockIPAMRepository()
	service := NewService(repo)

	tests := []struct {
		name      string
		baseCIDR  string
		maxPeers  int
		count     int
		expectErr bool
	}{
		{
			name:      "small network",
			baseCIDR:  "10.0.0.0/8",
			maxPeers:  10,
			count:     1,
			expectErr: false,
		},
		{
			name:      "medium network",
			baseCIDR:  "10.0.0.0/8",
			maxPeers:  100,
			count:     2,
			expectErr: false,
		},
		{
			name:      "large network",
			baseCIDR:  "10.0.0.0/8",
			maxPeers:  1000,
			count:     1,
			expectErr: false,
		},
		{
			name:      "multiple suggestions",
			baseCIDR:  "10.0.0.0/8",
			maxPeers:  50,
			count:     5,
			expectErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			prefixLen, cidrs, err := service.SuggestCIDRs(context.Background(), tt.baseCIDR, tt.maxPeers, tt.count)

			if tt.expectErr && err == nil {
				t.Error("Expected error but got none")
			}
			if !tt.expectErr && err != nil {
				t.Errorf("Unexpected error: %v", err)
			}

			if !tt.expectErr {
				if len(cidrs) != tt.count {
					t.Errorf("Expected %d CIDRs, got %d", tt.count, len(cidrs))
				}

				if prefixLen < 8 || prefixLen > 32 {
					t.Errorf("Expected prefix length between 8-32, got %d", prefixLen)
				}

				// Verify all returned CIDRs are valid
				for i, cidr := range cidrs {
					if cidr == "" {
						t.Errorf("CIDR %d is empty", i)
					}
				}
			}
		})
	}
}

func TestService_SuggestCIDRs_InvalidInput(t *testing.T) {
	repo := newMockIPAMRepository()
	service := NewService(repo)

	tests := []struct {
		name     string
		baseCIDR string
		maxPeers int
		count    int
	}{
		{
			name:     "zero maxPeers",
			baseCIDR: "10.0.0.0/8",
			maxPeers: 0,
			count:    1,
		},
		{
			name:     "negative maxPeers",
			baseCIDR: "10.0.0.0/8",
			maxPeers: -1,
			count:    1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, _, err := service.SuggestCIDRs(context.Background(), tt.baseCIDR, tt.maxPeers, tt.count)

			if err == nil {
				t.Error("Expected error but got none")
			}
		})
	}
}

func TestService_SuggestCIDRs_CountHandling(t *testing.T) {
	repo := newMockIPAMRepository()
	service := NewService(repo)

	tests := []struct {
		name          string
		count         int
		expectedCount int
	}{
		{
			name:          "zero count defaults to 1",
			count:         0,
			expectedCount: 1,
		},
		{
			name:          "negative count defaults to 1",
			count:         -1,
			expectedCount: 1,
		},
		{
			name:          "positive count",
			count:         3,
			expectedCount: 3,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, cidrs, err := service.SuggestCIDRs(context.Background(), "10.0.0.0/8", 10, tt.count)

			if err != nil {
				t.Errorf("Unexpected error: %v", err)
			}

			if len(cidrs) != tt.expectedCount {
				t.Errorf("Expected %d CIDRs, got %d", tt.expectedCount, len(cidrs))
			}
		})
	}
}

func TestService_SuggestCIDRs_PrefixLengthCalculation(t *testing.T) {
	repo := newMockIPAMRepository()
	service := NewService(repo)

	tests := []struct {
		name             string
		maxPeers         int
		expectedMinHosts int
	}{
		{
			name:             "small network - 10 peers",
			maxPeers:         10,
			expectedMinHosts: 10,
		},
		{
			name:             "medium network - 100 peers",
			maxPeers:         100,
			expectedMinHosts: 100,
		},
		{
			name:             "large network - 1000 peers",
			maxPeers:         1000,
			expectedMinHosts: 1000,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			prefixLen, cidrs, err := service.SuggestCIDRs(context.Background(), "10.0.0.0/8", tt.maxPeers, 1)

			if err != nil {
				t.Errorf("Unexpected error: %v", err)
			}

			if len(cidrs) != 1 {
				t.Errorf("Expected 1 CIDR, got %d", len(cidrs))
			}

			// Calculate usable hosts for the returned prefix length
			usableHosts := (1 << (32 - prefixLen)) - 2

			if usableHosts < tt.expectedMinHosts {
				t.Errorf("Prefix length %d provides %d hosts, but need at least %d",
					prefixLen, usableHosts, tt.expectedMinHosts)
			}

			// Ensure prefix length is bounded
			if prefixLen < 8 {
				t.Errorf("Prefix length %d is too small (< 8)", prefixLen)
			}
			if prefixLen > 32 {
				t.Errorf("Prefix length %d is too large (> 32)", prefixLen)
			}
		})
	}
}

func TestService_SuggestCIDRs_RepositoryInteraction(t *testing.T) {
	repo := newMockIPAMRepository()
	service := NewService(repo)

	// Test that the service interacts with the repository correctly
	_, cidrs, err := service.SuggestCIDRs(context.Background(), "10.0.0.0/8", 10, 2)

	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}

	if len(cidrs) != 2 {
		t.Errorf("Expected 2 CIDRs, got %d", len(cidrs))
	}

	// Verify that prefixes were created and then deleted in the repository
	// (This is based on the current implementation that ensures and then deletes prefixes)
	if len(repo.prefixes) != 0 {
		t.Errorf("Expected repository to be clean after operation, but has %d prefixes", len(repo.prefixes))
	}
}

// Test edge case where maxPeers is extremely large
func TestService_SuggestCIDRs_ExtremelyLargePeers(t *testing.T) {
	repo := newMockIPAMRepository()
	service := NewService(repo)

	// Test with a very large number of peers that would require a very small prefix
	_, cidrs, err := service.SuggestCIDRs(context.Background(), "10.0.0.0/8", 1000000, 1)

	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}

	if len(cidrs) != 1 {
		t.Errorf("Expected 1 CIDR, got %d", len(cidrs))
	}
}
