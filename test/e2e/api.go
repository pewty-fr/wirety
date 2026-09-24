//go:build e2e

// Package e2e is a black-box end-to-end harness for Wirety. It stands up a real
// stack with testcontainers — postgres, Dex (OIDC), the wirety-server, a jump
// peer running the agent, and plain WireGuard peers — and asserts the iptables
// state the agent programs from server-side policy, WireGuard connectivity, the
// captive-portal auth flow, and private DNS resolution.
//
// These tests require a Linux host with Docker and the WireGuard kernel module
// (privileged containers create real wg interfaces). They are gated behind the
// `e2e` build tag and are NOT part of `go test ./...`. See README.md.
package e2e

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// apiClient is a minimal Wirety REST client. It keeps its own JSON structs
// rather than importing the server module, so the harness stays decoupled from
// server internals and exercises the wire contract exactly as an external
// caller would.
type apiClient struct {
	baseURL string // e.g. http://127.0.0.1:32768/api/v1
	bearer  string // OIDC id_token used as Authorization: Bearer
	http    *http.Client
}

func newAPIClient(baseURL, bearer string) *apiClient {
	return &apiClient{
		baseURL: baseURL,
		bearer:  bearer,
		http:    &http.Client{Timeout: 30 * time.Second},
	}
}

// ---- wire types (subset of the server API) --------------------------------

type network struct {
	ID           string `json:"id"`
	Name         string `json:"name"`
	CIDR         string `json:"cidr"`
	CIDRv6       string `json:"cidr_v6,omitempty"`
	DomainSuffix string `json:"domain_suffix,omitempty"`
}

type peer struct {
	ID         string `json:"id"`
	Name       string `json:"name"`
	Address    string `json:"address"`
	AddressV6  string `json:"address_v6,omitempty"`
	PublicKey  string `json:"public_key"`
	Endpoint   string `json:"endpoint,omitempty"`
	ListenPort int    `json:"listen_port,omitempty"`
	Token      string `json:"token,omitempty"` // enrollment token (only present on create)
	IsJump     bool   `json:"is_jump"`
	UseAgent   bool   `json:"use_agent"`
	OwnerID    string `json:"owner_id,omitempty"`
}

type group struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type policyRule struct {
	Direction  string `json:"direction"`   // "input" | "output"
	Action     string `json:"action"`      // "allow" | "deny"
	TargetType string `json:"target_type"` // "cidr" | "peer" | "group"
	Target     string `json:"target"`
}

type policy struct {
	ID    string       `json:"id"`
	Name  string       `json:"name"`
	Rules []policyRule `json:"rules"`
}

type route struct {
	ID                string `json:"id"`
	Name              string `json:"name"`
	DestinationCIDR   string `json:"destination_cidr,omitempty"`
	DestinationCIDRv6 string `json:"destination_cidr_v6,omitempty"`
	JumpPeerID        string `json:"jump_peer_id"`
}

type dnsMapping struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	IPAddress   string `json:"ip_address,omitempty"`
	IPv6Address string `json:"ip_address_v6,omitempty"`
}

// ---- request helper --------------------------------------------------------

func (c *apiClient) do(ctx context.Context, method, path string, body, out any) error {
	var rdr io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return fmt.Errorf("marshal %s %s: %w", method, path, err)
		}
		rdr = bytes.NewReader(b)
	}
	req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, rdr)
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	if c.bearer != "" {
		req.Header.Set("Authorization", "Bearer "+c.bearer)
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("%s %s: %w", method, path, err)
	}
	defer func() { _ = resp.Body.Close() }()
	respBody, _ := io.ReadAll(resp.Body)
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("%s %s: status %d: %s", method, path, resp.StatusCode, string(respBody))
	}
	if out != nil && len(respBody) > 0 {
		if err := json.Unmarshal(respBody, out); err != nil {
			return fmt.Errorf("%s %s: decode response: %w (body=%s)", method, path, err, string(respBody))
		}
	}
	return nil
}

// ---- endpoints -------------------------------------------------------------

func (c *apiClient) health(ctx context.Context) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+"/health", nil)
	if err != nil {
		return err
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return err
	}
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("health: status %d", resp.StatusCode)
	}
	return nil
}

func (c *apiClient) createNetwork(ctx context.Context, req network) (network, error) {
	var out network
	err := c.do(ctx, http.MethodPost, "/networks", req, &out)
	return out, err
}

func (c *apiClient) getNetwork(ctx context.Context, networkID string) (network, error) {
	var out network
	err := c.do(ctx, http.MethodGet, "/networks/"+networkID, nil, &out)
	return out, err
}

// user is the subset of /users/me the harness needs.
type user struct {
	ID    string `json:"id"`
	Email string `json:"email"`
}

func (c *apiClient) me(ctx context.Context) (user, error) {
	var out user
	err := c.do(ctx, http.MethodGet, "/users/me", nil, &out)
	return out, err
}

// createPeerReq mirrors domain.PeerCreateRequest for the fields the harness uses.
type createPeerReq struct {
	Name       string `json:"name"`
	IsJump     bool   `json:"is_jump"`
	UseAgent   bool   `json:"use_agent"`
	Endpoint   string `json:"endpoint,omitempty"`
	ListenPort int    `json:"listen_port,omitempty"`
	OwnerID    string `json:"owner_id,omitempty"`
}

func (c *apiClient) createPeer(ctx context.Context, networkID string, req createPeerReq) (peer, error) {
	var out peer
	err := c.do(ctx, http.MethodPost, "/networks/"+networkID+"/peers", req, &out)
	return out, err
}

// peerConfig fetches the generated WireGuard config for a peer.
func (c *apiClient) peerConfig(ctx context.Context, networkID, peerID string) (string, error) {
	var out struct {
		Config string `json:"config"`
	}
	err := c.do(ctx, http.MethodGet, "/networks/"+networkID+"/peers/"+peerID+"/config", nil, &out)
	return out.Config, err
}

func (c *apiClient) createGroup(ctx context.Context, networkID, name string) (group, error) {
	var out group
	err := c.do(ctx, http.MethodPost, "/networks/"+networkID+"/groups", map[string]string{"name": name}, &out)
	return out, err
}

func (c *apiClient) addPeerToGroup(ctx context.Context, networkID, groupID, peerID string) error {
	return c.do(ctx, http.MethodPost, "/networks/"+networkID+"/groups/"+groupID+"/peers/"+peerID, nil, nil)
}

// createPolicyReq mirrors domain.PolicyCreateRequest.
type createPolicyReq struct {
	Name  string       `json:"name"`
	Rules []policyRule `json:"rules"`
}

func (c *apiClient) createPolicy(ctx context.Context, networkID string, req createPolicyReq) (policy, error) {
	var out policy
	err := c.do(ctx, http.MethodPost, "/networks/"+networkID+"/policies", req, &out)
	return out, err
}

func (c *apiClient) attachPolicyToGroup(ctx context.Context, networkID, groupID, policyID string) error {
	return c.do(ctx, http.MethodPost, "/networks/"+networkID+"/groups/"+groupID+"/policies/"+policyID, nil, nil)
}

type createRouteReq struct {
	Name              string `json:"name"`
	DestinationCIDR   string `json:"destination_cidr,omitempty"`
	DestinationCIDRv6 string `json:"destination_cidr_v6,omitempty"`
	JumpPeerID        string `json:"jump_peer_id"`
}

func (c *apiClient) createRoute(ctx context.Context, networkID string, req createRouteReq) (route, error) {
	var out route
	err := c.do(ctx, http.MethodPost, "/networks/"+networkID+"/routes", req, &out)
	return out, err
}

func (c *apiClient) attachRouteToGroup(ctx context.Context, networkID, groupID, routeID string) error {
	return c.do(ctx, http.MethodPost, "/networks/"+networkID+"/groups/"+groupID+"/routes/"+routeID, nil, nil)
}

type createDNSReq struct {
	Name        string `json:"name"`
	IPAddress   string `json:"ip_address,omitempty"`
	IPv6Address string `json:"ip_address_v6,omitempty"`
}

func (c *apiClient) createDNS(ctx context.Context, networkID, routeID string, req createDNSReq) (dnsMapping, error) {
	var out dnsMapping
	err := c.do(ctx, http.MethodPost, "/networks/"+networkID+"/routes/"+routeID+"/dns", req, &out)
	return out, err
}
