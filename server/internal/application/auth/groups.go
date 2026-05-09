package auth

import (
	"errors"
	"strings"

	domainauth "wirety/internal/domain/auth"
)

// ErrNotInAuthorizedGroup is returned by GetOrCreateUser when group-based access
// control is configured and the user does not belong to any authorised group.
// Callers should respond with 403 and a generic message — do not expose claim details
// to the HTTP client.
var ErrNotInAuthorizedGroup = errors.New("not in authorized group")

// ParseGroups splits a comma-separated group list, trims whitespace, and drops empty entries.
func ParseGroups(raw string) []string {
	if raw == "" {
		return nil
	}
	var groups []string
	for _, g := range strings.Split(raw, ",") {
		if t := strings.TrimSpace(g); t != "" {
			groups = append(groups, t)
		}
	}
	return groups
}

// DetermineRoleFromGroups returns the Wirety role for a user given their group memberships.
//
// Rules:
//   - If the user belongs to any group in adminGroups → RoleAdministrator (admin wins).
//   - If the user belongs to any group in requiredGroups → RoleUser.
//   - If requiredGroups is empty → RoleUser (no user-group filter).
//   - Otherwise → ("", false): the user must be rejected.
//
// Both slices may be nil/empty: callers should fall through to the first-user-is-admin
// logic when neither is configured.
func DetermineRoleFromGroups(userGroups, adminGroups, requiredGroups []string) (domainauth.Role, bool) {
	set := make(map[string]bool, len(userGroups))
	for _, g := range userGroups {
		set[g] = true
	}

	for _, ag := range adminGroups {
		if set[ag] {
			return domainauth.RoleAdministrator, true
		}
	}

	if len(requiredGroups) == 0 {
		return domainauth.RoleUser, true
	}
	for _, rg := range requiredGroups {
		if set[rg] {
			return domainauth.RoleUser, true
		}
	}
	return "", false
}

// ExtractGroupsClaim extracts a group list from raw JWT map claims.
// Handles JSON arrays ([]interface{}), comma-separated strings, and space-separated strings.
func ExtractGroupsClaim(raw map[string]interface{}, claimName string) []string {
	if claimName == "" {
		return nil
	}
	val, ok := raw[claimName]
	if !ok {
		return nil
	}
	switch v := val.(type) {
	case []interface{}:
		groups := make([]string, 0, len(v))
		for _, item := range v {
			if s, ok := item.(string); ok && s != "" {
				groups = append(groups, s)
			}
		}
		return groups
	case string:
		if strings.Contains(v, ",") {
			return ParseGroups(v)
		}
		return strings.Fields(v)
	}
	return nil
}
