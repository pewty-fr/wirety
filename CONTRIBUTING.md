# Contributing to Wirety

Thank you for your interest in contributing to Wirety! This document provides guidelines and instructions for contributing to the project.

## Getting Started

### Prerequisites

- Go 1.21+ (for server and agent development)
- Node.js 20+ and npm (for frontend development)
- Docker and Docker Compose (for testing)
- WireGuard installed on your system
- Git

### Development Setup

1. **Clone the repository**
```bash
git clone https://github.com/pewty/wirety.git
cd wirety
```

2. **Server Development**
```bash
cd server
go mod download
make run
```

The server runs on `http://localhost:8080` by default.

3. **Frontend Development**
```bash
cd front
npm install
npm run dev
```

The frontend runs on `http://localhost:5173` with hot-reload enabled.

4. **Agent Development**
```bash
cd agent
go mod download
make build
```

5. **Documentation**
```bash
cd doc
npm install
npm run start
```

Documentation site runs on `http://localhost:3000`.

## Project Structure

```
wirety/
├── server/          # Go server (API, orchestration)
├── agent/           # Go agent (peer automation)
├── front/           # React frontend (dashboard)
├── doc/             # Docusaurus documentation
├── helm/            # Kubernetes Helm chart
└── dex/             # Example OIDC provider setup
```

## How to Contribute

### Reporting Issues

- Check existing issues before creating a new one
- Use the issue templates when available
- Provide clear reproduction steps
- Include relevant logs and system information
- Tag issues appropriately (bug, enhancement, documentation)

### Submitting Pull Requests

1. **Fork the repository** and create your branch from `main`
```bash
git checkout -b feature/your-feature-name
```

2. **Make your changes**
   - Follow the code style guidelines (see below)
   - Add tests for new features
   - Update documentation as needed
   - Ensure all tests pass

3. **Commit your changes**
   - Use conventional commit messages:
     ```
     feat: add peer isolation toggle
     fix: resolve IPAM allocation race condition
     docs: update OIDC guide with Keycloak example
     chore: update dependencies
     ```
   - Reference issues: `fixes #123` or `relates to #456`

4. **Push to your fork**
```bash
git push origin feature/your-feature-name
```

5. **Open a Pull Request**
   - Provide a clear description of the changes
   - Reference related issues
   - Update the CHANGELOG if applicable
   - Request review from maintainers

### Code Style Guidelines

#### Go (Server & Agent)
- Follow standard Go formatting (`gofmt`, `goimports`)
- Use meaningful variable and function names
- Add comments for exported functions and types
- Write unit tests for business logic
- Keep functions small and focused
- Use structured logging

#### TypeScript/React (Frontend)
- Use TypeScript strict mode
- Follow React best practices (hooks, functional components)
- Use meaningful component and variable names
- Write type-safe code (avoid `any` when possible)
- Keep components small and reusable
- Add PropTypes or TypeScript interfaces

#### Documentation
- Use clear, concise language
- Include code examples where relevant
- Update screenshots if UI changes
- Follow Markdown best practices
- Test documentation locally before submitting

## Testing

### Server Tests
```bash
cd server
go test ./...
```

### Frontend Tests
```bash
cd front
npm run test
```

### Integration Tests
```bash
# Start services with Docker Compose
docker-compose up -d
# Run integration tests
./test/integration.sh
```

## Development Workflow

1. **Issue Discussion**: Discuss significant changes in an issue before implementation
2. **Branch Naming**: Use descriptive branch names (e.g., `feature/ipam-pool`, `fix/websocket-leak`)
3. **Small PRs**: Keep pull requests focused and reasonably sized
4. **Code Review**: Address review comments promptly
5. **CI/CD**: Ensure all CI checks pass before requesting review

## Release Process

Releases are managed by project maintainers using release-please:
- Semantic versioning (MAJOR.MINOR.PATCH)
- Automated CHANGELOG generation
- Tagged releases with GitHub releases
- Container images published to registry

## Community Guidelines

### Code of Conduct

- Be respectful and inclusive
- Welcome newcomers and help them get started
- Focus on constructive feedback
- Respect different viewpoints and experiences
- Accept responsibility for mistakes

### Communication Channels

- **GitHub Issues**: Bug reports, feature requests, questions
- **Pull Requests**: Code contributions and reviews
- **Discussions**: General questions, ideas, and community chat

## License

By contributing to Wirety, you agree that your contributions will be licensed under the same license as the project (see LICENSE file).

## Questions?

If you have questions about contributing:
- Open a GitHub issue with the `question` label
- Check existing documentation in the `doc/` folder
- Review closed issues and PRs for similar questions

## Recognition

Contributors are recognized in:
- Git commit history
- Release notes (when applicable)
- Project documentation

Thank you for contributing to Wirety! 🚀
