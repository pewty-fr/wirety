package api

import (
	"errors"
	"fmt"
	"net/http"
	"testing"

	domain "wirety/internal/domain/network"
	"wirety/internal/infrastructure/validation"
)

func TestServiceErrorStatus(t *testing.T) {
	for msg, want := range map[string]int{
		"route not found: no rows": http.StatusNotFound,
		"validation failed: at least one of destination_cidr or destination_cidr_v6 must remain set": http.StatusBadRequest,
		"peer is not a jump peer":                                      http.StatusBadRequest,
		"ip_address validation failed: not in the route":               http.StatusBadRequest,
		"failed to update route: connection refused":                   http.StatusInternalServerError,
		"failed to create group: group name already exists in network": http.StatusConflict,
	} {
		if got := serviceErrorStatus(errors.New(msg)); got != want {
			t.Errorf("serviceErrorStatus(%q) = %d, want %d", msg, got, want)
		}
	}
	for _, err := range []error{
		fmt.Errorf("%w: 10.0.0.0/16 overlaps 10.0.0.0/24, the range of network “prod”", domain.ErrCIDROverlap),
		fmt.Errorf("%w: network “tiny” has no free IPv4 address left in 10.0.0.0/30", domain.ErrNetworkFull),
	} {
		if got := serviceErrorStatus(err); got != http.StatusConflict {
			t.Errorf("serviceErrorStatus(%q) = %d, want 409", err, got)
		}
	}
}

// Services wrap validation errors; they are still client errors.
func TestIsValidationErrorSeesWrappedErrors(t *testing.T) {
	if !isValidationError(fmt.Errorf("invalid peer name: %w", validation.ErrInvalidDNSName)) {
		t.Error("a wrapped invalid-name error is not a validation error")
	}
	if !isValidationError(fmt.Errorf("additional_allowed_ips: %w", domain.ErrInvalidCIDR)) {
		t.Error("a wrapped invalid-CIDR error is not a validation error")
	}
	if isValidationError(errors.New("failed to update peer: connection refused")) {
		t.Error("a storage failure is a validation error")
	}
}
