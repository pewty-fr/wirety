package policy

// PeerPolicy holds peer filtering attributes.
type PeerPolicy struct {
	ID       string `json:"id"`
	IP       string `json:"ip"`
	Isolated bool   `json:"isolated"`
}

// JumpPolicy delivered to jump agent to enforce isolation & ACL.
type JumpPolicy struct {
	IP         string       `json:"ip"`
	Peers      []PeerPolicy `json:"peers"`
	ACLBlocked []string     `json:"acl_blocked"`
}
