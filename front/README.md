# Wirety Front-End

React Native mobile application for managing WireGuard networks using the Wirety server API.

## Features

- **Network Management**: List, add, update, and view WireGuard networks with CIDR helper
- **Peer Management**: Add/update regular and jump server peers with full configuration
- **IPAM**: View IP allocation across all networks with filtering
- **Real-time Config**: Generate and view WireGuard configs and enrollment tokens

## Setup

```bash
# Install dependencies
npm install

# Start the development server
npm start

# Run on specific platform
npm run ios
npm run android
npm run web
```

## Configuration

Update the API base URL in `src/services/api.ts`:

```typescript
const client = new ApiClient('http://your-server:8080/api/v1');
```

## Architecture

- **Services**: API client layer
- **Screens**: Feature-organized UI components
- **Components**: Reusable UI elements
- **Types**: TypeScript API type definitions
- **Utils**: Validation and helper functions

## Navigation

The app uses React Navigation with bottom tabs:
- **Networks**: Network and peer management
- **IPAM**: IP allocation overview

## Screens

### Networks
- List networks with search and pagination
- Add network with CIDR helper
- View/update network details
- Access peer list from network

### Peers
- List peers with filters (jump/regular, isolated, etc.)
- Add regular peer or jump server
- View peer details with config/token access
- Update peer configuration

### IPAM
- List all IP allocations across networks
- Filter by network, IP, or peer
- View allocation status

## Development

The app uses:
- Expo for React Native development
- React Navigation for routing
- React Native Paper for UI components
- Axios for API communication
- TypeScript for type safety
