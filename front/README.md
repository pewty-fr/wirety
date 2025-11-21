# Wirety Web Administration Console

A modern web administration interface for Wirety WireGuard network management, inspired by Scaleway's console design.

## ✨ Features

- 🌐 **Networks Management** - View all WireGuard networks with stats
- 💻 **Peers Management** - Monitor peers across all networks  
- 📊 **IPAM** - IP address allocation tracking
- 🔒 **Security** - Security incident monitoring and resolution
- 👥 **Users** - User management and permissions

## 🚀 Quick Start

```bash
# Install dependencies
npm install

# Start development server
npm run dev

# Build for production
npm run build
```

The app will be available at **http://localhost:5173**

## 🎨 Design

Clean, modern interface inspired by Scaleway's console:
- Sidebar navigation
- Card-based layouts
- Professional color scheme  
- Responsive tables
- Search and pagination

## 🔌 API Configuration

Default API URL: `http://localhost:8080/api/v1`

To change it, edit `src/api/client.ts`:
```typescript
constructor(baseURL: string = 'YOUR_API_URL') {
```

## 📁 Project Structure

```
src/
├── api/client.ts              # API client
├── types/index.ts             # TypeScript types
├── components/
│   ├── Layout.tsx             # Main layout with sidebar
│   └── PageHeader.tsx         # Page header component
├── pages/
│   ├── networks/NetworksPage.tsx
│   ├── peers/PeersPage.tsx
│   ├── ipam/IPAMPage.tsx
│   ├── security/SecurityPage.tsx
│   └── users/UsersPage.tsx
└── App.tsx                    # Main app with routing
```

## 🛠️ Tech Stack

- React 19 + TypeScript
- Vite (Rolldown)
- Tailwind CSS v4
- React Router v7
- Axios

## 📸 Screenshots

Navigate between sections using the sidebar:
- Networks: Grid view with search
- Peers: Table view with type indicators
- IPAM: IP allocation tracking with stats
- Security: Incident cards with resolution
- Users: User list with roles

##License

See main project license.
