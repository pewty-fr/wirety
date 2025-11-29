package policy

// JumpPolicy delivered to jump agent to enforce isolation & ACL.
type JumpPolicy struct {
	IP            string   `json:"ip"`
	IPTablesRules []string `json:"iptables_rules"` // Generated iptables rules from policies
}
