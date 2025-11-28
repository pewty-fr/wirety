package policy

// PeerPolicy holds peer filtering attributes.
type PeerPolicy struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	IP       string `json:"ip"`
	Isolated bool   `json:"isolated"`
	UseAgent bool   `json:"use_agent"`
}

// JumpPolicy delivered to jump agent to enforce isolation & ACL.
type JumpPolicy struct {
	IP            string       `json:"ip"`
	IPTablesRules []string     `json:"iptables_rules"` // Generated iptables rules from policies
	Peers         []PeerPolicy `json:"peers"`
	ACLBlocked    []string     `json:"acl_blocked"` // Deprecated: kept for backward compatibility
}
