package ports

// ConfigWriterPort defines capability to write and apply WireGuard config.
type ConfigWriterPort interface {
	WriteAndApply(cfg string) error
}

// DNSStarterPort defines capability to start DNS server with given domain and peers.
type DNSStarterPort interface {
	Start(addr string) error
}

// WebSocketClientPort defines capability to connect and receive messages.
type WebSocketClientPort interface {
	Connect(url string) error
	ReadMessage() ([]byte, error)
	Close() error
}
