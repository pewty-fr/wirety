# Wirety Server

A WireGuard network management server built with hexagonal architecture in Go.

## Features

- **Network Management**: Create and manage WireGuard mesh networks
- **Peer Management**: Add, update, and remove peers from networks
- **ACL Support**: Configure access control rules to control peer communication
- **Configuration Generation**: Automatically generate WireGuard configurations for each peer
- **WebSocket Support**: Real-time configuration updates via WebSocket
- **RESTful API**: Full REST API with Swagger documentation
- **Hexagonal Architecture**: Clean separation of concerns with domain-driven design

## Architecture

```
├── cmd/server/           # Application entrypoint
├── internal/
│   ├── domain/network/   # Domain entities (Peer, Network, ACL)
│   ├── application/      # Business logic (Service layer)
│   └── adapters/
│       ├── api/          # HTTP API adapter (Gin + Swagger)
│       └── db/memory/    # In-memory repository
└── pkg/wireguard/        # WireGuard config generation utilities
```

## Installation

### Prerequisites

- Go 1.21 or higher
- Git

### Setup

1. Clone the repository:
```bash
git clone <repository-url>
cd wirety/server
```

2. Install dependencies:
```bash
go mod download
```

3. Build the application:
```bash
go build -o wirety-server ./cmd/server
```

## Usage

### Running the Server

```bash
# Run with default port (8080)
./wirety-server

# Run with custom port
HTTP_PORT=9000 ./wirety-server
```

### API Endpoints

#### Networks

- `POST /api/v1/networks` - Create a new network
- `GET /api/v1/networks` - List all networks
- `GET /api/v1/networks/:networkId` - Get a specific network
- `DELETE /api/v1/networks/:networkId` - Delete a network

#### Peers

- `POST /api/v1/networks/:networkId/peers` - Add a peer to a network
- `GET /api/v1/networks/:networkId/peers` - List all peers in a network
- `GET /api/v1/networks/:networkId/peers/:peerId` - Get a specific peer
- `PUT /api/v1/networks/:networkId/peers/:peerId` - Update a peer
- `DELETE /api/v1/networks/:networkId/peers/:peerId` - Delete a peer
- `GET /api/v1/networks/:networkId/peers/:peerId/config` - Get WireGuard config for a peer

#### ACL

- `GET /api/v1/networks/:networkId/acl` - Get ACL configuration
- `PUT /api/v1/networks/:networkId/acl` - Update ACL configuration

#### WebSocket

- `GET /api/v1/ws/:networkId/:peerId` - WebSocket endpoint for real-time config updates

### Example: Creating a Network and Adding Peers

```bash
# Create a network
curl -X POST http://localhost:8080/api/v1/networks \
  -H "Content-Type: application/json" \
  -d '{
    "name": "my-network",
    "cidr": "10.0.0.0/16",
    "domain": "wirety.local"
  }'

# Add a jump server peer
curl -X POST http://localhost:8080/api/v1/networks/<network-id>/peers \
  -H "Content-Type: application/json" \
  -d '{
    "name": "jump-server",
    "public_key": "<public-key>",
    "endpoint": "195.154.74.61:51820",
    "is_jump": true,
    "jump_nat_interface": "eth0"
  }'

# Add an isolated peer (user device)
curl -X POST http://localhost:8080/api/v1/networks/<network-id>/peers \
  -H "Content-Type: application/json" \
  -d '{
    "name": "user-laptop",
    "public_key": "<public-key>",
    "is_isolated": true
  }'

# Get WireGuard configuration for a peer
curl http://localhost:8080/api/v1/networks/<network-id>/peers/<peer-id>/config
```

## Configuration

### Environment Variables

- `HTTP_PORT` - HTTP server port (default: 8080)

## Development

### Project Structure

- **Domain Layer** (`internal/domain/network/`): Contains core business entities and interfaces
  - `peer.go` - Peer entity and request models
  - `network.go` - Network entity and business logic
  - `acl.go` - Access control list logic
  - `repository.go` - Repository interface

- **Application Layer** (`internal/application/network/`): Contains business logic
  - `service.go` - Network service with use cases

- **Adapters**: External interfaces
  - **API Adapter** (`internal/adapters/api/`): REST API and WebSocket handlers
  - **DB Adapter** (`internal/adapters/db/memory/`): In-memory repository implementation

- **Infrastructure** (`pkg/wireguard/`): Shared utilities
  - `config.go` - WireGuard configuration generation

### Adding a Database

To replace the in-memory repository with a real database:

1. Create a new adapter in `internal/adapters/db/` (e.g., `postgres/`)
2. Implement the `network.Repository` interface
3. Update `cmd/server/main.go` to use the new repository

### Running Tests

```bash
go test ./...
```

### Generating Swagger Documentation

```bash
# Install swag
go install github.com/swaggo/swag/cmd/swag@latest

# Generate docs
swag init -g cmd/server/main.go -o docs/swagger
```

## Network Isolation Rules

- **Isolated peers** (user devices) cannot communicate directly with each other
- **Non-isolated peers** (resources/servers) can communicate with everyone
- **Jump servers** route traffic for the entire network
- **ACL rules** can further restrict communication between peers

## Roadmap

- [ ] PostgreSQL adapter for persistent storage
- [ ] JWT authentication
- [ ] Multi-tenancy support
- [ ] Metrics and monitoring
- [ ] Automated key generation
- [ ] Network topology visualization
- [ ] Peer health checks

## License

MIT License

## Contributing

Contributions are welcome! Please open an issue or submit a pull request.
