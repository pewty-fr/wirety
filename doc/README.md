# Wirety Documentation

This directory contains the Docusaurus site for the Wirety project documentation.

## Structure

```
doc/
	website/
		package.json          # Docusaurus dependencies & scripts
		docusaurus.config.js  # Site configuration (title, navbar, footer)
		sidebars.js           # Sidebar ordering
		src/css/custom.css    # Theme overrides
		docs/                 # Markdown source files
			intro.md            # Get Started guide
			network.md          # Network architecture
			peers.md            # Peer types and behavior
			ipam.md             # IP allocation system
			incidents.md        # Security incident model
			agent.md            # Agent usage and internals
			server.md           # Server configuration & data
			api-reference.md    # Complete API documentation
			migration-guide.md  # Migration from legacy system
			user-guide.md       # Step-by-step user guides
			groups-policies-routes-overview.md  # New architecture overview
			guides/             # Task-oriented guides
				internet-access.md
				isolate-peer.md
				oidc.md
				groups-management.md  # Groups management guide
```

## Local Development

Install dependencies and start the dev server:

```bash
cd doc/website
npm install
npm run start
```

Then open http://localhost:3000.

## Build

```bash
NODE_OPTIONS="--no-webstorage" npm run build
npm run serve   # Preview production build
```

## Deployment

Configure `url` and `baseUrl` in `docusaurus.config.js` to match your hosting environment. If using GitHub Pages, set `organizationName` and `projectName` appropriately and use `npm run deploy`.

## New Documentation (v2.0)

The following documentation has been added for the groups, policies, and routes architecture:

### Core Documentation
- **[Groups, Policies, Routes Overview](docs/groups-policies-routes-overview.md)** - High-level overview of the new architecture
- **[API Reference](docs/api-reference.md)** - Complete API documentation for all endpoints
- **[Migration Guide](docs/migration-guide.md)** - Migrate from legacy ACL and peer flag system
- **[User Guide](docs/user-guide.md)** - Step-by-step guides for common tasks

### Detailed Guides
- **[Groups Management](docs/guides/groups-management.md)** - Comprehensive groups management guide

### Key Changes in v2.0
- ✅ Groups for organizing peers
- ✅ Policies for iptables-based traffic filtering
- ✅ Routes for external network access
- ✅ DNS mappings for route networks
- ✅ Default groups for automatic assignment
- ❌ Legacy ACL system removed
- ❌ Peer flags (is_isolated, full_encapsulation) removed

**Migration Required:** See [Migration Guide](docs/migration-guide.md)

## Contributing

1. Add or edit markdown under `docs/`.
2. Update `sidebars.js` if you create new top-level pages.
3. Run `npm run start` to preview.
4. Open a PR.

## Pending Items

Some placeholders (Helm repo URLs, Swagger path) will be updated once charts and API docs are published.

---
Copyright © Wirety

