package api

import (
	"context"
	"encoding/json"
	"net/http"
	"sync"

	"wirety/internal/application/network"
	"wirety/internal/config"
	domain "wirety/internal/domain/network"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"github.com/rs/zerolog/log"
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
}

// WebSocketManager manages WebSocket connections for peer configuration updates
type WebSocketManager struct {
	service     *network.Service
	authConfig  *config.AuthConfig
	connections map[string]map[string]*websocket.Conn // networkID -> peerID -> conn
	mu          sync.RWMutex
}

// NewWebSocketManager creates a new WebSocket manager
func NewWebSocketManager(service *network.Service, authConfig *config.AuthConfig) *WebSocketManager {
	return &WebSocketManager{
		service:     service,
		authConfig:  authConfig,
		connections: make(map[string]map[string]*websocket.Conn),
	}
}

// Register adds a connection to the manager
func (m *WebSocketManager) Register(networkID, peerID string, conn *websocket.Conn) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.connections[networkID]; !exists {
		m.connections[networkID] = make(map[string]*websocket.Conn)
	}
	m.connections[networkID][peerID] = conn
	log.Info().Str("network_id", networkID).Str("peer_id", peerID).Msg("WebSocket connection registered")
}

// Unregister removes a connection from the manager
func (m *WebSocketManager) Unregister(networkID, peerID string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if peers, exists := m.connections[networkID]; exists {
		delete(peers, peerID)
		if len(peers) == 0 {
			delete(m.connections, networkID)
		}
	}
	log.Info().Str("network_id", networkID).Str("peer_id", peerID).Msg("WebSocket connection unregistered")
}

// IsConnected checks if a peer has an active WebSocket connection
func (m *WebSocketManager) IsConnected(networkID, peerID string) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if peers, exists := m.connections[networkID]; exists {
		_, connected := peers[peerID]
		return connected
	}
	return false
}

// HandleWebSocket handles WebSocket connections for peer configuration updates
func (h *Handler) HandleWebSocket(c *gin.Context) {
	networkID := c.Param("networkId")
	peerID := c.Param("peerId")

	// Upgrade HTTP connection to WebSocket
	conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		log.Error().Err(err).Msg("Failed to upgrade connection")
		return
	}
	defer func() {
		h.wsManager.Unregister(networkID, peerID)
		_ = conn.Close()
	}()

	log.Info().
		Str("network_id", networkID).
		Str("peer_id", peerID).
		Msg("WebSocket connection established")

	// Register connection
	h.wsManager.Register(networkID, peerID, conn)

	// Send initial configuration
	config, err := h.service.GeneratePeerConfig(c.Request.Context(), networkID, peerID)
	if err != nil {
		log.Error().Err(err).Msg("Failed to generate initial config")
		return
	}

	if err := conn.WriteMessage(websocket.TextMessage, []byte(config)); err != nil {
		log.Error().Err(err).Msg("Failed to send initial config")
		return
	}

	// Keep connection alive and listen for close
	for {
		_, _, err := conn.ReadMessage()
		if err != nil {
			log.Info().
				Str("network_id", networkID).
				Str("peer_id", peerID).
				Msg("WebSocket connection closed")
			break
		}
	}
}

// HandleWebSocketToken handles WebSocket connections authenticated by enrollment token (?token=...)
func (h *Handler) HandleWebSocketToken(c *gin.Context) {
	token := c.Query("token")
	if token == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "token required"})
		return
	}
	networkID, peer, err := h.service.ResolveAgentToken(c.Request.Context(), token)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid token"})
		return
	}
	conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		log.Error().Err(err).Msg("Failed to upgrade connection (token)")
		return
	}
	defer func() {
		h.wsManager.Unregister(networkID, peer.ID)
		_ = conn.Close()
	}()

	log.Info().Str("network_id", networkID).Str("peer_id", peer.ID).Msg("WebSocket token connection established")

	// Register connection
	h.wsManager.Register(networkID, peer.ID, conn)

	cfg, dnsCfg, policy, err := h.service.GeneratePeerConfigWithDNS(c.Request.Context(), networkID, peer.ID)
	if err != nil {
		log.Error().Err(err).Msg("Failed to generate initial config (token)")
		return
	}

	// Get whitelist if this is a jump peer
	var whitelist []string
	if peer.IsJump {
		whitelist, err = h.service.GetCaptivePortalWhitelist(c.Request.Context(), networkID, peer.ID)
		if err != nil {
			log.Warn().Err(err).Str("network_id", networkID).Str("peer_id", peer.ID).Msg("Failed to get whitelist")
			whitelist = []string{}
		}
	}

	// Get OAuth issuer from config
	oauthIssuer := ""
	if h.wsManager.authConfig != nil && h.wsManager.authConfig.Enabled {
		oauthIssuer = h.wsManager.authConfig.IssuerURL
	}

	msg := struct {
		Config      string      `json:"config"`
		DNS         interface{} `json:"dns,omitempty"`
		Policy      interface{} `json:"policy,omitempty"`
		Whitelist   []string    `json:"whitelist,omitempty"`
		OAuthIssuer string      `json:"oauth_issuer,omitempty"`
	}{
		Config:      cfg,
		DNS:         dnsCfg,
		Policy:      policy,
		Whitelist:   whitelist,
		OAuthIssuer: oauthIssuer,
	}
	data, _ := json.Marshal(msg)
	if err := conn.WriteMessage(websocket.TextMessage, data); err != nil {
		log.Error().Err(err).Msg("Failed to send initial config (token)")
		return
	}
	for {
		msgType, message, err := conn.ReadMessage()
		if err != nil {
			log.Info().Str("network_id", networkID).Str("peer_id", peer.ID).Msg("WebSocket token connection closed")
			break
		}

		// Process heartbeat messages from agent
		if msgType == websocket.TextMessage {
			var heartbeat domain.AgentHeartbeat
			if err := json.Unmarshal(message, &heartbeat); err != nil {
				log.Warn().Err(err).Msg("Failed to parse heartbeat message")
				continue
			}

			// Process the heartbeat
			if err := h.service.ProcessAgentHeartbeat(c.Request.Context(), networkID, peer.ID, &heartbeat); err != nil {
				log.Error().Err(err).Msg("Failed to process agent heartbeat")
			} else {
				log.Debug().
					Str("network_id", networkID).
					Str("peer_id", peer.ID).
					Str("hostname", heartbeat.Hostname).
					Interface("peer_endpoints", heartbeat.PeerEndpoints).
					Msg("Agent heartbeat processed")
			}
		}
	}
}

// NotifyPeerUpdate sends updated configuration to a specific peer via WebSocket
func (m *WebSocketManager) NotifyPeerUpdate(networkID, peerID string) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if peers, exists := m.connections[networkID]; exists {
		if conn, exists := peers[peerID]; exists {
			ctx := context.Background()
			cfg, dnsCfg, policy, err := m.service.GeneratePeerConfigWithDNS(ctx, networkID, peerID)
			if err != nil {
				log.Error().Err(err).Str("network_id", networkID).Str("peer_id", peerID).Msg("Failed to generate config for update")
				return
			}

			// Get peer information for metadata
			peer, err := m.service.GetPeer(ctx, networkID, peerID)
			if err != nil {
				log.Error().Err(err).Str("network_id", networkID).Str("peer_id", peerID).Msg("Failed to get peer info for update")
				return
			}

			// Get whitelist if this is a jump peer
			var whitelist []string
			if peer.IsJump {
				whitelist, err = m.service.GetCaptivePortalWhitelist(ctx, networkID, peerID)
				if err != nil {
					log.Warn().Err(err).Str("network_id", networkID).Str("peer_id", peerID).Msg("Failed to get whitelist")
					whitelist = []string{}
				}
			}

			// Get OAuth issuer from config
			oauthIssuer := ""
			if m.authConfig != nil && m.authConfig.Enabled {
				oauthIssuer = m.authConfig.IssuerURL
			}

			msg := struct {
				Config      string      `json:"config"`
				DNS         interface{} `json:"dns,omitempty"`
				Policy      interface{} `json:"policy,omitempty"`
				PeerID      string      `json:"peer_id"`
				PeerName    string      `json:"peer_name"`
				Whitelist   []string    `json:"whitelist,omitempty"`
				OAuthIssuer string      `json:"oauth_issuer,omitempty"`
			}{
				Config:      cfg,
				DNS:         dnsCfg,
				Policy:      policy,
				PeerID:      peer.ID,
				PeerName:    peer.Name,
				Whitelist:   whitelist,
				OAuthIssuer: oauthIssuer,
			}
			data, _ := json.Marshal(msg)
			if err := conn.WriteMessage(websocket.TextMessage, data); err != nil {
				log.Error().Err(err).Str("network_id", networkID).Str("peer_id", peerID).Msg("Failed to send config update")
			} else {
				log.Info().Str("network_id", networkID).Str("peer_id", peerID).Str("peer_name", peer.Name).Msg("Config update sent")
			}
		}
	}
}

// NotifyNetworkPeers sends updated configuration to all connected peers in a network
func (m *WebSocketManager) NotifyNetworkPeers(networkID string) {
	m.mu.RLock()
	peerIDs := make([]string, 0)
	if peers, exists := m.connections[networkID]; exists {
		for peerID := range peers {
			peerIDs = append(peerIDs, peerID)
		}
	}
	m.mu.RUnlock()

	// Generate and send config for each connected peer
	for _, peerID := range peerIDs {
		m.NotifyPeerUpdate(networkID, peerID)
	}
}
