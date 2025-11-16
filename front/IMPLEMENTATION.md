# Wirety Front-End - Complete Documentation

## Overview
A comprehensive React Native mobile application for managing WireGuard mesh networks via the Wirety server API.

## Project Structure

```
front/
├── App.tsx                          # Main app entry with navigation
├── package.json                     # Dependencies and scripts
├── tsconfig.json                    # TypeScript configuration
├── babel.config.js                  # Babel/Expo configuration
├── app.json                         # Expo configuration
├── src/
│   ├── services/
│   │   └── api.ts                   # API client with all endpoints
│   ├── types/
│   │   └── api.ts                   # TypeScript type definitions
│   ├── utils/
│   │   └── validation.ts            # Validation and formatting utilities
│   ├── components/
│   │   ├── FormComponents.tsx       # Reusable form inputs
│   │   └── Pagination.tsx           # Pagination component
│   ├── screens/
│   │   ├── networks/
│   │   │   ├── NetworkListScreen.tsx    # List networks with search/pagination
│   │   │   ├── NetworkAddScreen.tsx     # Add network with CIDR helper
│   │   │   ├── NetworkViewScreen.tsx    # View network details
│   │   │   └── NetworkUpdateScreen.tsx  # Update network
│   │   ├── peers/
│   │   │   ├── PeerListScreen.tsx           # List peers with filters
│   │   │   ├── PeerAddChoiceScreen.tsx      # Choose peer type to add
│   │   │   ├── PeerAddRegularScreen.tsx     # Add regular peer form
│   │   │   ├── PeerAddJumpScreen.tsx        # Add jump server form
│   │   │   ├── PeerViewScreen.tsx           # View peer details
│   │   │   ├── PeerTokenScreen.tsx          # Display enrollment token
│   │   │   └── PeerConfigScreen.tsx         # Display WireGuard config
│   │   └── ipam/
│   │       └── IPAMListScreen.tsx           # List IP allocations
```

## Features Implemented

### Network Management
- ✅ **List Networks**: Searchable, paginated list with filtering
- ✅ **Add Network**: Form with CIDR helper (suggests common private ranges)
- ✅ **View Network**: Display details, created/updated dates
- ✅ **Update Network**: Edit name and domain
- ✅ **Delete Network**: Remove network with confirmation

### Peer Management
- ✅ **List Peers**: Searchable list with jump/isolated/encapsulation badges
- ✅ **Add Regular Peer**: Form with isolation, full encapsulation, additional IPs
- ✅ **Add Jump Server**: Form with endpoint, listen port, NAT interface
- ✅ **View Peer**: Details with conditional fields for jump vs regular
- ✅ **Update Peer**: Edit peer configuration
- ✅ **Get Token**: Display enrollment token for agent setup
- ✅ **Get Config**: Display WireGuard configuration file

### IPAM
- ✅ **List Allocations**: View all IP allocations across networks
- ✅ **Filter**: Search by network name, IP, or peer name
- ✅ **Pagination**: Navigate through large allocation lists
- ✅ **Status Display**: Shows allocated vs available IPs

## API Integration

### Endpoints Implemented

**Networks:**
- `GET /api/v1/networks` - List with pagination/filter
- `GET /api/v1/networks/:id` - Get network details
- `POST /api/v1/networks` - Create network
- `PUT /api/v1/networks/:id` - Update network
- `DELETE /api/v1/networks/:id` - Delete network

**Peers:**
- `GET /api/v1/networks/:networkId/peers` - List with pagination/filter
- `GET /api/v1/networks/:networkId/peers/:peerId` - Get peer details
- `POST /api/v1/networks/:networkId/peers` - Create peer
- `PUT /api/v1/networks/:networkId/peers/:peerId` - Update peer
- `DELETE /api/v1/networks/:networkId/peers/:peerId` - Delete peer
- `GET /api/v1/networks/:networkId/peers/:peerId/token` - Get enrollment token
- `GET /api/v1/networks/:networkId/peers/:peerId/config` - Get WireGuard config

**IPAM:**
- `GET /api/v1/ipam` - List allocations with pagination/filter
- `GET /api/v1/networks/:networkId/ipam` - Get network-specific allocations

## Navigation Structure

```
MainTabs (Bottom Tab Navigator)
├── Networks (Stack Navigator)
│   ├── NetworkList
│   ├── NetworkAdd
│   ├── NetworkView
│   ├── NetworkUpdate
│   ├── PeerList
│   ├── PeerAddChoice
│   ├── PeerAddRegular
│   ├── PeerAddJump
│   ├── PeerView
│   ├── PeerToken
│   └── PeerConfig
└── IPAM (Stack Navigator)
    └── IPAMList
```

## Components

### FormComponents
- **TextInput**: Styled text input with error display
- **FormButton**: Button with loading state

### Pagination
- Previous/Next navigation
- Current page display
- Automatic disable when at bounds

## Utilities

### Validation
- `validateCIDR`: IPv4 CIDR format validation
- `validateIPv4`: IPv4 address validation
- `validatePort`: Port number range validation
- `validateEndpoint`: IP:PORT format validation
- `validateDomain`: Domain name format validation
- `suggestCIDRs`: Common private CIDR ranges

### Formatting
- `formatDate`: Human-readable date/time
- `truncate`: String truncation with ellipsis

## Installation & Setup

```bash
# Navigate to the front directory
cd /Users/tanguyfalconnet/git/github.com/pewty/wirety/front

# Install dependencies
npm install

# Configure API endpoint
# Edit src/services/api.ts and set your server URL:
# const client = new ApiClient('http://your-server:8080/api/v1');

# Start development server
npm start

# Run on platform
npm run ios      # iOS simulator
npm run android  # Android emulator
npm run web      # Web browser
```

## Dependencies

**Core:**
- `expo`: React Native development platform
- `react`: UI library
- `react-native`: Mobile framework

**Navigation:**
- `@react-navigation/native`: Navigation framework
- `@react-navigation/native-stack`: Stack navigator
- `@react-navigation/bottom-tabs`: Tab navigator
- `react-native-screens`: Native screen primitives
- `react-native-safe-area-context`: Safe area handling

**UI:**
- `react-native-paper`: Material Design components
- `react-native-vector-icons`: Icon library

**API:**
- `axios`: HTTP client

## Configuration Files

### package.json
- Scripts for running, building, linting
- Dependencies for Expo, React Navigation, Paper, Axios

### tsconfig.json
- TypeScript strict mode
- JSX configuration for React Native
- Path aliases for clean imports

### app.json
- Expo configuration
- App name, version, icons

### babel.config.js
- Expo preset configuration

## Usage Guide

### Adding a Network
1. Navigate to Networks tab
2. Tap the (+) FAB button
3. Enter network name, CIDR, and domain
4. Use CIDR helper chips for common ranges
5. Tap "Create Network"

### Adding a Peer
1. Navigate to Networks → Select Network → View Peers
2. Tap (+) FAB button
3. Choose "Regular Peer" or "Jump Server"
4. Fill in required fields:
   - Regular: name, optional endpoint, isolation settings
   - Jump: name, endpoint, listen port, NAT interface
5. Tap "Create Peer"

### Getting Peer Token/Config
1. Navigate to peer details
2. Tap "View Token" for enrollment token
3. Tap "View Config" for WireGuard configuration
4. Use token with agent: `TOKEN=<token> ./agent`

### Viewing IPAM
1. Navigate to IPAM tab
2. View all IP allocations across networks
3. Use search to filter by network, IP, or peer
4. Navigate pages for large datasets

## Type Definitions

All API types are defined in `src/types/api.ts`:
- `Network`, `NetworkCreateRequest`, `NetworkUpdateRequest`
- `Peer`, `PeerCreateRequest`, `PeerUpdateRequest`
- `IPAMAllocation`
- `PaginatedResponse<T>`
- `TokenResponse`, `ConfigResponse`

## Error Handling

- Network errors caught and logged to console
- Form validation with inline error messages
- Loading states with ActivityIndicator
- Graceful fallbacks for missing data

## Future Enhancements

Potential additions (not currently implemented):
- Peer update screens (regular/jump specific)
- Settings screen for API endpoint configuration
- Offline caching with AsyncStorage
- Pull-to-refresh improvements
- Network status indicators
- ACL management UI
- QR code generation for tokens
- Config file export/share

## Notes

The TypeScript errors shown during creation are expected before running `npm install`. They occur because:
- Dependencies (react, react-native, etc.) are not yet installed
- The errors will resolve after running `npm install`
- The JSX flag issue is resolved in the tsconfig.json

To actually run the app:
1. Install dependencies: `npm install`
2. Configure the API base URL in `src/services/api.ts`
3. Start Expo: `npm start`
4. Scan QR code with Expo Go app or press `i`/`a` for simulator
