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
			guides/             # Task-oriented guides
				internet-access.md
				isolate-peer.md
				oidc.md
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

## Contributing

1. Add or edit markdown under `docs/`.
2. Update `sidebars.js` if you create new top-level pages.
3. Run `npm run start` to preview.
4. Open a PR.

## Pending Items

Some placeholders (Helm repo URLs, Swagger path) will be updated once charts and API docs are published.

---
Copyright © Wirety
