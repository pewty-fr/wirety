package main

import (
	"context"
	"fmt"
	"strings"

	"wirety/pkg/wireguard"

	goipam "github.com/metal-stack/go-ipam"
)

type WireguardConfiguration struct {
	Interface WireguardInterface
	Peers     []WireguardPeer
}

type WireguardInterface struct {
	PrivateKey string
	Address    string
	ListenPort int
	DNS        []string
	PostUp     []string
	PostDown   []string
}

type WireguardPeer struct {
	PublicKey           string
	PresharedKey        string
	AllowedIPs          []string
	Endpoint            string
	PersistentKeepalive int
}

type Peer struct {
	Name string
	Jump struct {
		Enable       bool
		NatInterface string
		Address      string
		MaxPeers     int
		Domain       string
		DNS          []string
	}
	NetworkName            string
	Isolation              bool
	WireguardConfiguration WireguardConfiguration
	PrivateKey             string
	PublicKey              string
	Address                string
	Port                   int
	FullEncapsulation      bool
}

type Network struct {
	CIDR   string
	Peers  []Peer
	Jump   Peer
	Domain string
}

var (
	peers = []Peer{
		{
			Name: "jump",
			Jump: struct {
				Enable       bool
				NatInterface string
				Address      string
				MaxPeers     int
				Domain       string
				DNS          []string
			}{
				Enable:       true,
				NatInterface: "eth0",
				Address:      "195.154.74.61",
				MaxPeers:     50,
				Domain:       "wirety.local",
				DNS:          []string{"8.8.8.8", "8.8.4.4"},
			},
			Port: 51820,
		},
		{
			Name:              "peer1",
			Isolation:         true,
			NetworkName:       "wirety.local",
			Port:              51820,
			FullEncapsulation: true,
		},
		{
			Name:        "peer2",
			Isolation:   true,
			NetworkName: "wirety.local",
			Port:        51820,
		},
		{
			Name:        "resource1",
			Isolation:   false,
			NetworkName: "wirety.local",
			Port:        51820,
		},
	}
)

func main() {
	defaultCIDR := "10.0.0.0/24"
	bgCtx := context.Background()
	ipam := goipam.New(bgCtx)
	prefix, err := ipam.NewPrefix(bgCtx, defaultCIDR)
	if err != nil {
		panic(err)
	}

	networks := make([]Network, 0)
	// Generate a network per jump server
	for _, peer := range peers {
		if peer.Jump.Enable {
			// Calculate appropriate prefix length based on peer count
			prefixLength := calculatePrefixLength(peer.Jump.MaxPeers)

			cp1, err := ipam.AcquireChildPrefix(bgCtx, prefix.Cidr, uint8(prefixLength))
			if err != nil {
				panic(err)
			}

			networkPeers := make([]Peer, 0)
			for _, p := range peers {
				if peer.Jump.Domain != p.NetworkName {
					continue
				}
				networkPeers = append(networkPeers, p)
			}

			// Generate WireGuard key pair for the jump server
			privateKey, publicKey, err := wireguard.GenerateKeyPair()
			if err != nil {
				panic(err)
			}

			fmt.Printf("Generated keys for %s - Public: %s\n", peer.Name, publicKey)

			peer.PrivateKey = privateKey
			peer.PublicKey = publicKey

			ipJump, err := ipam.AcquireIP(bgCtx, cp1.Cidr)
			if err != nil {
				panic(err)
			}

			peer.Address = ipJump.IP.String()
			peer.WireguardConfiguration = WireguardConfiguration{
				Interface: WireguardInterface{
					PrivateKey: privateKey,
					Address:    fmt.Sprintf("%s/%d", peer.Address, prefixLength),
					ListenPort: peer.Port,
					DNS:        peer.Jump.DNS,
				},
				Peers: []WireguardPeer{},
			}

			if peer.Jump.NatInterface != "" {
				peer.WireguardConfiguration.Interface.PostUp = []string{
					fmt.Sprintf("iptables -A FORWARD -i %%i -j ACCEPT"),
					fmt.Sprintf("iptables -t nat -A POSTROUTING -o %s -j MASQUERADE", peer.Jump.NatInterface),
					"sysctl -w net.ipv4.ip_forward=1",
				}
				peer.WireguardConfiguration.Interface.PostDown = []string{
					fmt.Sprintf("iptables -D FORWARD -i %%i -j ACCEPT"),
					fmt.Sprintf("iptables -t nat -D POSTROUTING -o %s -j MASQUERADE", peer.Jump.NatInterface),
					"sysctl -w net.ipv4.ip_forward=1",
				}
			}

			fmt.Printf("%v\n", networkPeers)

			for k, p := range networkPeers {
				// Generate WireGuard key pair for the peer
				privKey, pubKey, err := wireguard.GenerateKeyPair()
				if err != nil {
					panic(err)
				}

				fmt.Printf("Generated keys for %s - Public: %s\n", p.Name, pubKey)

				presharedKey, err := wireguard.GeneratePresharedKey()
				if err != nil {
					panic(err)
				}

				networkPeers[k].PrivateKey = privKey
				networkPeers[k].PublicKey = pubKey
				ipPeer, err := ipam.AcquireIP(bgCtx, cp1.Cidr)
				if err != nil {
					panic(err)
				}
				networkPeers[k].Address = ipPeer.IP.String()

				allowedIPs := []string{cp1.Cidr}
				if p.FullEncapsulation {
					allowedIPs = []string{"0.0.0.0/0", "::/0"}
				}
				networkPeers[k].WireguardConfiguration = WireguardConfiguration{
					Interface: WireguardInterface{
						PrivateKey: privKey,
						Address:    fmt.Sprintf("%s/%d", networkPeers[k].Address, prefixLength),
						ListenPort: networkPeers[k].Port,
						DNS:        peer.Jump.DNS,
					},
					Peers: []WireguardPeer{
						{
							PublicKey:    peer.PublicKey,
							AllowedIPs:   allowedIPs,
							Endpoint:     fmt.Sprintf("%s:%d", peer.Jump.Address, peer.Port),
							PresharedKey: presharedKey,
						},
					},
				}
				peer.WireguardConfiguration.Peers = append(peer.WireguardConfiguration.Peers, WireguardPeer{
					PublicKey:    pubKey,
					AllowedIPs:   []string{fmt.Sprintf("%s/32", ipPeer.IP)},
					PresharedKey: presharedKey,
				})
			}
			fmt.Printf("%v\n", networkPeers)

			// handle isolation peers
			for _, p := range networkPeers {
				if p.Isolation {
					peer.WireguardConfiguration.Interface.PostUp = append(peer.WireguardConfiguration.Interface.PostUp, fmt.Sprintf("iptables -A FORWARD -i %%i -d %s/32 -j DROP", p.Address))
					peer.WireguardConfiguration.Interface.PostDown = append(peer.WireguardConfiguration.Interface.PostDown, fmt.Sprintf("iptables -D FORWARD -i %%i -d %s/32 -j DROP", p.Address))
				}
			}

			network := Network{
				CIDR:   cp1.Cidr,
				Peers:  networkPeers,
				Domain: peer.Jump.Domain,
				Jump:   peer,
			}
			networks = append(networks, network)
			fmt.Printf("Network %s for %s: %s (supports up to %d hosts)\n", network.Domain, peer.Name, network.CIDR, (1<<(32-prefixLength))-2)
		}
	}

	// Output WireGuard configurations
	for _, network := range networks {
		fmt.Printf("\n=== Network: %s (%s) ===\n", network.Domain, network.CIDR)
		fmt.Printf("Jump Server: %s (%s)\n", network.Jump.Name, network.Jump.Address)
		generateWireGuardConfig(network.Jump.WireguardConfiguration)
		for _, peer := range network.Peers {
			fmt.Printf("\n--- Peer: %s (%s) ---\n", peer.Name, peer.Address)
			generateWireGuardConfig(peer.WireguardConfiguration)
		}
	}

}

// calculatePrefixLength determines the appropriate CIDR prefix length based on peer count
// Returns prefix length that can accommodate the number of peers with some headroom
func calculatePrefixLength(peerCount int) int {
	// Add 50% headroom for future growth
	requiredHosts := peerCount

	// Account for network and broadcast addresses
	requiredHosts += 2

	// Calculate bits needed: 2^bits >= requiredHosts
	bits := 0
	for (1 << bits) < requiredHosts {
		bits++
	}

	// Prefix length is 32 - bits (for IPv4)
	prefixLength := 32 - bits

	// Ensure minimum /30 (2 usable hosts) and maximum /20 (4094 usable hosts)
	if prefixLength > 30 {
		prefixLength = 30
	}
	if prefixLength < 20 {
		prefixLength = 20
	}

	return prefixLength
}

func generateWireGuardConfig(wgc WireguardConfiguration) {
	fmt.Println("[Interface]")
	fmt.Printf("PrivateKey = %s\n", wgc.Interface.PrivateKey)
	fmt.Printf("Address = %s\n", wgc.Interface.Address)
	fmt.Printf("ListenPort = %d\n", wgc.Interface.ListenPort)
	if len(wgc.Interface.DNS) > 0 {
		fmt.Printf("DNS = %s\n", strings.Join(wgc.Interface.DNS, ", "))
	}
	if len(wgc.Interface.PostUp) > 0 {
		fmt.Printf("PostUp = %s\n", strings.Join(wgc.Interface.PostUp, "; "))
	}
	if len(wgc.Interface.PostDown) > 0 {
		fmt.Printf("PostDown = %s\n", strings.Join(wgc.Interface.PostDown, "; "))
	}
	fmt.Println()

	for _, peer := range wgc.Peers {
		fmt.Println("[Peer]")
		fmt.Printf("PublicKey = %s\n", peer.PublicKey)
		if peer.PresharedKey != "" {
			fmt.Printf("PresharedKey = %s\n", peer.PresharedKey)
		}
		fmt.Printf("AllowedIPs = %s\n", strings.Join(peer.AllowedIPs, ", "))
		if peer.Endpoint != "" {
			fmt.Printf("Endpoint = %s\n", peer.Endpoint)
		}
		if peer.PersistentKeepalive > 0 {
			fmt.Printf("PersistentKeepalive = %d\n", peer.PersistentKeepalive)
		}
		fmt.Println()
	}

}
