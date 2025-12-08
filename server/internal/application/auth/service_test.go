package auth

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"encoding/base64"
	"fmt"
	"math/big"
	"testing"
	"time"

	"wirety/internal/config"
	"wirety/internal/domain/auth"

	"github.com/golang-jwt/jwt/v5"
)

// mockAuthRepository implements auth.Repository for testing
type mockAuthRepository struct {
	users                 map[string]*auth.User
	sessions              map[string]*auth.Session
	defaultPermissions    *auth.DefaultNetworkPermissions
	firstUserReturnsError bool
}

func newMockAuthRepository() *mockAuthRepository {
	return &mockAuthRepository{
		users:    make(map[string]*auth.User),
		sessions: make(map[string]*auth.Session),
	}
}

func (m *mockAuthRepository) GetUser(userID string) (*auth.User, error) {
	if user, exists := m.users[userID]; exists {
		return user, nil
	}
	return nil, fmt.Errorf("user not found")
}

func (m *mockAuthRepository) GetUserByEmail(email string) (*auth.User, error) {
	for _, user := range m.users {
		if user.Email == email {
			return user, nil
		}
	}
	return nil, fmt.Errorf("user not found")
}

func (m *mockAuthRepository) CreateUser(user *auth.User) error {
	m.users[user.ID] = user
	return nil
}

func (m *mockAuthRepository) UpdateUser(user *auth.User) error {
	if _, exists := m.users[user.ID]; !exists {
		return fmt.Errorf("user not found")
	}
	m.users[user.ID] = user
	return nil
}

func (m *mockAuthRepository) DeleteUser(userID string) error {
	delete(m.users, userID)
	return nil
}

func (m *mockAuthRepository) ListUsers() ([]*auth.User, error) {
	var users []*auth.User
	for _, user := range m.users {
		users = append(users, user)
	}
	return users, nil
}

func (m *mockAuthRepository) GetFirstUser() (*auth.User, error) {
	if m.firstUserReturnsError {
		return nil, fmt.Errorf("no users found")
	}
	for _, user := range m.users {
		return user, nil
	}
	return nil, fmt.Errorf("no users found")
}

func (m *mockAuthRepository) GetDefaultPermissions() (*auth.DefaultNetworkPermissions, error) {
	if m.defaultPermissions == nil {
		return nil, fmt.Errorf("no default permissions set")
	}
	return m.defaultPermissions, nil
}

func (m *mockAuthRepository) SetDefaultPermissions(perms *auth.DefaultNetworkPermissions) error {
	m.defaultPermissions = perms
	return nil
}

func (m *mockAuthRepository) CreateSession(session *auth.Session) error {
	m.sessions[session.SessionHash] = session
	return nil
}

func (m *mockAuthRepository) GetSession(sessionHash string) (*auth.Session, error) {
	if session, exists := m.sessions[sessionHash]; exists {
		return session, nil
	}
	return nil, fmt.Errorf("session not found")
}

func (m *mockAuthRepository) UpdateSession(session *auth.Session) error {
	if _, exists := m.sessions[session.SessionHash]; !exists {
		return fmt.Errorf("session not found")
	}
	m.sessions[session.SessionHash] = session
	return nil
}

func (m *mockAuthRepository) DeleteSession(sessionHash string) error {
	delete(m.sessions, sessionHash)
	return nil
}

func (m *mockAuthRepository) DeleteUserSessions(userID string) error {
	for hash, session := range m.sessions {
		if session.UserID == userID {
			delete(m.sessions, hash)
		}
	}
	return nil
}

func (m *mockAuthRepository) CleanupExpiredSessions() error {
	now := time.Now()
	for hash, session := range m.sessions {
		if session.RefreshTokenExpiresAt.Before(now) {
			delete(m.sessions, hash)
		}
	}
	return nil
}

func TestNewService(t *testing.T) {
	cfg := &config.AuthConfig{
		Enabled:      true,
		IssuerURL:    "https://example.com",
		ClientID:     "test-client",
		ClientSecret: "test-secret",
		JWKSCacheTTL: 3600,
	}
	repo := newMockAuthRepository()

	service := NewService(cfg, repo)

	if service == nil {
		t.Error("Expected service to be created, got nil")
		return
	}

	if service.config != cfg {
		t.Error("Expected service to use provided config")
	}

	if service.userRepo != repo {
		t.Error("Expected service to use provided repository")
	}
}

func TestService_ValidateToken_AuthDisabled(t *testing.T) {
	cfg := &config.AuthConfig{
		Enabled: false,
	}
	repo := newMockAuthRepository()
	service := NewService(cfg, repo)

	_, err := service.ValidateToken(context.Background(), "test-token")

	if err == nil {
		t.Error("Expected error when auth is disabled")
	}

	if err.Error() != "authentication is not enabled" {
		t.Errorf("Expected specific error message, got: %v", err)
	}
}

func TestService_ValidateToken_InvalidTokenFormat(t *testing.T) {
	cfg := &config.AuthConfig{
		Enabled:   true,
		IssuerURL: "https://example.com",
	}
	repo := newMockAuthRepository()
	service := NewService(cfg, repo)

	_, err := service.ValidateToken(context.Background(), "invalid-token")

	if err == nil {
		t.Error("Expected error for invalid token format")
	}
}

func TestService_ValidateToken_MissingKid(t *testing.T) {
	cfg := &config.AuthConfig{
		Enabled:   true,
		IssuerURL: "https://example.com",
	}
	repo := newMockAuthRepository()
	service := NewService(cfg, repo)

	// Create token without kid header
	privateKey, _ := rsa.GenerateKey(rand.Reader, 2048)
	claims := jwt.MapClaims{
		"sub": "test-user",
		"iss": "https://example.com",
		"exp": time.Now().Add(time.Hour).Unix(),
	}

	token := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
	// Don't set kid header
	tokenString, _ := token.SignedString(privateKey)

	_, err := service.ValidateToken(context.Background(), tokenString)

	if err == nil {
		t.Error("Expected error for missing kid header")
	}
}

func TestService_GetOrCreateUser_ExistingUser(t *testing.T) {
	cfg := &config.AuthConfig{Enabled: true}
	repo := newMockAuthRepository()
	service := NewService(cfg, repo)

	// Create existing user
	existingUser := &auth.User{
		ID:          "test-user-123",
		Email:       "test@example.com",
		Name:        "Test User",
		Role:        auth.RoleUser,
		CreatedAt:   time.Now().Add(-time.Hour),
		LastLoginAt: time.Now().Add(-time.Minute),
	}
	repo.users[existingUser.ID] = existingUser

	claims := &auth.OIDCClaims{
		Subject: "test-user-123",
		Email:   "test@example.com",
		Name:    "Test User Updated",
	}

	user, err := service.GetOrCreateUser(context.Background(), claims)

	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}

	if user.ID != existingUser.ID {
		t.Errorf("Expected user ID %s, got %s", existingUser.ID, user.ID)
	}

	// Check that LastLoginAt was updated (allow for same second due to timing)
	if user.LastLoginAt.Before(existingUser.LastLoginAt) {
		t.Error("Expected LastLoginAt to be updated or at least not go backwards")
	}
}

func TestService_GetOrCreateUser_FirstUser(t *testing.T) {
	cfg := &config.AuthConfig{Enabled: true}
	repo := newMockAuthRepository()
	repo.firstUserReturnsError = true // Simulate no existing users
	service := NewService(cfg, repo)

	claims := &auth.OIDCClaims{
		Subject: "first-user-123",
		Email:   "admin@example.com",
		Name:    "First User",
	}

	user, err := service.GetOrCreateUser(context.Background(), claims)

	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}

	if user.Role != auth.RoleAdministrator {
		t.Errorf("Expected first user to be administrator, got %s", user.Role)
	}

	if user.ID != claims.Subject {
		t.Errorf("Expected user ID %s, got %s", claims.Subject, user.ID)
	}

	if user.Email != claims.Email {
		t.Errorf("Expected user email %s, got %s", claims.Email, user.Email)
	}
}

func TestService_GetOrCreateUser_NewUserWithDefaults(t *testing.T) {
	cfg := &config.AuthConfig{Enabled: true}
	repo := newMockAuthRepository()
	service := NewService(cfg, repo)

	// Add an existing user so this won't be the first user
	existingUser := &auth.User{
		ID:   "existing-user",
		Role: auth.RoleAdministrator,
	}
	repo.users[existingUser.ID] = existingUser

	// Set default permissions
	defaultPerms := &auth.DefaultNetworkPermissions{
		DefaultRole:               auth.RoleUser,
		DefaultAuthorizedNetworks: []string{"net1", "net2"},
	}
	repo.defaultPermissions = defaultPerms

	claims := &auth.OIDCClaims{
		Subject: "new-user-123",
		Email:   "newuser@example.com",
		Name:    "New User",
	}

	user, err := service.GetOrCreateUser(context.Background(), claims)

	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}

	if user.Role != auth.RoleUser {
		t.Errorf("Expected user role %s, got %s", auth.RoleUser, user.Role)
	}

	if len(user.AuthorizedNetworks) != 2 {
		t.Errorf("Expected 2 authorized networks, got %d", len(user.AuthorizedNetworks))
	}
}

func TestService_RefreshAccessToken_AuthDisabled(t *testing.T) {
	cfg := &config.AuthConfig{
		Enabled: false,
	}
	repo := newMockAuthRepository()
	service := NewService(cfg, repo)

	_, _, err := service.RefreshAccessToken(context.Background(), "refresh-token")

	if err == nil {
		t.Error("Expected error when auth is disabled")
	}
}

func TestJwkToPublicKey_Success(t *testing.T) {
	privateKey, _ := rsa.GenerateKey(rand.Reader, 2048)
	publicKey := &privateKey.PublicKey

	// Convert to JWK format
	nBytes := publicKey.N.Bytes()
	eBytes := big.NewInt(int64(publicKey.E)).Bytes()

	jwk := map[string]interface{}{
		"kty": "RSA",
		"n":   base64.RawURLEncoding.EncodeToString(nBytes),
		"e":   base64.RawURLEncoding.EncodeToString(eBytes),
	}

	convertedKey, err := jwkToPublicKey(jwk)

	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}

	if convertedKey.N.Cmp(publicKey.N) != 0 {
		t.Error("Converted key modulus doesn't match original")
	}

	if convertedKey.E != publicKey.E {
		t.Error("Converted key exponent doesn't match original")
	}
}

func TestJwkToPublicKey_InvalidKeyType(t *testing.T) {
	jwk := map[string]interface{}{
		"kty": "EC", // Not RSA
	}

	_, err := jwkToPublicKey(jwk)

	if err == nil {
		t.Error("Expected error for invalid key type")
	}
}

func TestJwkToPublicKey_MissingParameters(t *testing.T) {
	tests := []struct {
		name string
		jwk  map[string]interface{}
	}{
		{
			name: "missing n parameter",
			jwk: map[string]interface{}{
				"kty": "RSA",
				"e":   "AQAB",
			},
		},
		{
			name: "missing e parameter",
			jwk: map[string]interface{}{
				"kty": "RSA",
				"n":   "test",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := jwkToPublicKey(tt.jwk)
			if err == nil {
				t.Error("Expected error for missing parameter")
			}
		})
	}
}

// Test helper functions
func TestGetStringClaim(t *testing.T) {
	claims := jwt.MapClaims{
		"string_claim": "test_value",
		"int_claim":    123,
	}

	// Test valid string claim
	result := getStringClaim(claims, "string_claim")
	if result != "test_value" {
		t.Errorf("Expected 'test_value', got '%s'", result)
	}

	// Test non-string claim
	result = getStringClaim(claims, "int_claim")
	if result != "" {
		t.Errorf("Expected empty string for non-string claim, got '%s'", result)
	}

	// Test missing claim
	result = getStringClaim(claims, "missing_claim")
	if result != "" {
		t.Errorf("Expected empty string for missing claim, got '%s'", result)
	}
}

func TestGetBoolClaim(t *testing.T) {
	claims := jwt.MapClaims{
		"bool_claim":   true,
		"string_claim": "test",
	}

	// Test valid bool claim
	result := getBoolClaim(claims, "bool_claim")
	if result != true {
		t.Errorf("Expected true, got %v", result)
	}

	// Test non-bool claim
	result = getBoolClaim(claims, "string_claim")
	if result != false {
		t.Errorf("Expected false for non-bool claim, got %v", result)
	}
}

func TestGetInt64Claim(t *testing.T) {
	claims := jwt.MapClaims{
		"int_claim":    float64(123),
		"string_claim": "test",
	}

	// Test valid int claim
	result := getInt64Claim(claims, "int_claim")
	if result != 123 {
		t.Errorf("Expected 123, got %d", result)
	}

	// Test non-int claim
	result = getInt64Claim(claims, "string_claim")
	if result != 0 {
		t.Errorf("Expected 0 for non-int claim, got %d", result)
	}
}
