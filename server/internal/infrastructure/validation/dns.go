package validation

import (
	"errors"
	"fmt"
	"regexp"
	"strings"
)

var (
	// ErrInvalidDNSName indicates that a name doesn't conform to DNS naming conventions
	ErrInvalidDNSName = errors.New("name must follow DNS naming convention: lowercase letters, numbers, and hyphens only")

	// ErrNameTooLong indicates that a name exceeds the maximum length
	ErrNameTooLong = errors.New("name exceeds maximum length of 63 characters")

	// ErrNameEmpty indicates that a name is empty
	ErrNameEmpty = errors.New("name cannot be empty")

	// ErrNameStartsWithHyphen indicates that a name starts with a hyphen
	ErrNameStartsWithHyphen = errors.New("name cannot start with a hyphen")

	// ErrNameEndsWithHyphen indicates that a name ends with a hyphen
	ErrNameEndsWithHyphen = errors.New("name cannot end with a hyphen")
)

// dnsNameRegex matches valid DNS names (RFC 1123)
// - Must contain only lowercase letters (a-z), numbers (0-9), and hyphens (-)
// - Must not start or end with a hyphen
// - Maximum 63 characters per label
var dnsNameRegex = regexp.MustCompile(`^[a-z0-9]([a-z0-9-]*[a-z0-9])?$`)

// ValidateDNSName validates that a name follows DNS naming conventions
// according to RFC 1123 (subset of RFC 952)
func ValidateDNSName(name string) error {
	if name == "" {
		return ErrNameEmpty
	}

	// Check length (DNS label max is 63 characters)
	if len(name) > 63 {
		return ErrNameTooLong
	}

	// Check if it starts with hyphen
	if strings.HasPrefix(name, "-") {
		return ErrNameStartsWithHyphen
	}

	// Check if it ends with hyphen
	if strings.HasSuffix(name, "-") {
		return ErrNameEndsWithHyphen
	}

	// Check against regex pattern (must be lowercase already)
	if !dnsNameRegex.MatchString(name) {
		return ErrInvalidDNSName
	}

	return nil
}

// ValidateDNSHostname validates a DNS hostname that may contain multiple labels
// separated by dots (e.g. "prod", "prod.eu", "corp.example.com").
// Each individual label is validated as a standard DNS label (RFC 1123).
// Use this instead of ValidateDNSName whenever dots should be allowed.
func ValidateDNSHostname(name string) error {
	if name == "" {
		return ErrNameEmpty
	}

	// Overall DNS name limit is 253 characters
	if len(name) > 253 {
		return errors.New("name exceeds maximum length of 253 characters")
	}

	if strings.HasPrefix(name, ".") || strings.HasSuffix(name, ".") {
		return errors.New("name cannot start or end with a dot")
	}

	labels := strings.Split(name, ".")
	for _, label := range labels {
		if label == "" {
			return errors.New("name contains consecutive dots")
		}
		if err := ValidateDNSName(label); err != nil {
			return fmt.Errorf("invalid label %q: %w", label, err)
		}
	}

	return nil
}

// SanitizeDNSHostname converts a multi-label hostname to follow DNS naming conventions.
// It sanitizes each label individually, then re-joins them with dots.
func SanitizeDNSHostname(name string) string {
	if name == "" {
		return ""
	}

	// Lowercase and replace characters that are invalid even between dots
	name = strings.ToLower(name)
	invalidChars := regexp.MustCompile(`[^a-z0-9.\-]`)
	name = invalidChars.ReplaceAllString(name, "-")

	// Sanitize each label
	labels := strings.Split(name, ".")
	sanitized := make([]string, 0, len(labels))
	for _, label := range labels {
		label = strings.Trim(label, "-")
		if label == "" {
			continue
		}
		multipleHyphens := regexp.MustCompile(`-+`)
		label = multipleHyphens.ReplaceAllString(label, "-")
		if len(label) > 63 {
			label = strings.TrimSuffix(label[:63], "-")
		}
		sanitized = append(sanitized, label)
	}

	return strings.Join(sanitized, ".")
}

// SanitizeDNSName converts a name to follow DNS naming conventions
// This is useful for auto-correcting user input
func SanitizeDNSName(name string) string {
	if name == "" {
		return ""
	}

	// Convert to lowercase
	name = strings.ToLower(name)

	// Replace invalid characters with hyphens
	invalidChars := regexp.MustCompile(`[^a-z0-9-]`)
	name = invalidChars.ReplaceAllString(name, "-")

	// Remove leading/trailing hyphens and collapse multiple hyphens
	name = strings.Trim(name, "-")
	multipleHyphens := regexp.MustCompile(`-+`)
	name = multipleHyphens.ReplaceAllString(name, "-")

	// Truncate if too long
	if len(name) > 63 {
		name = name[:63]
		// Remove trailing hyphen if truncation created one
		name = strings.TrimSuffix(name, "-")
	}

	return name
}
