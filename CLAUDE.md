# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

This is a Go-based content monitoring system that analyzes website sitemaps, extracts keywords, and monitors content changes. The project is currently in the initial development phase with a comprehensive PRD document outlining the complete architecture.

## Development Commands

### Building and Running
```bash
# Build the project
go build -o sitemap-go .

# Run the project
go run main.go

# Run with specific config (when implemented)
./sitemap-go -config config/production.yaml
```

### Testing Commands
```bash
# Run all tests
go test ./...

# Run tests with coverage
go test -cover ./...

# Run benchmarks
go test -bench=. ./...

# Run specific test
go test -run TestSitemapParser ./pkg/parser
```

### Code Quality
```bash
# Format code
go fmt ./...

# Lint code (requires golangci-lint)
golangci-lint run

# Vet code for issues
go vet ./...

# Tidy modules
go mod tidy
```

## Project Architecture

### Core Design Principles
- **High Performance**: Uses goroutines and channels for concurrent processing
- **Worker Pool Pattern**: Manages concurrent task execution
- **Pipeline Architecture**: Data flows through processing stages
- **Plugin System**: Extensible architecture for different sitemap formats

### Key Components (as planned)
1. **Sitemap Parser**: Multi-format XML/RSS/TXT sitemap processing
2. **Keyword Extractor**: URL path analysis and keyword extraction
3. **API Manager**: Concurrent API client management with circuit breakers
4. **Storage Service**: Encrypted local file storage with caching
5. **Controller**: Orchestrates all services

### Architectural Patterns
- **Worker Pool**: For managing concurrent goroutines
- **Circuit Breaker**: For API resilience
- **Pipeline**: For data processing workflows
- **Factory Pattern**: For parser selection
- **Pool Pattern**: For object reuse (URLs, buffers)

## Project Structure (Planned)
```
├── cmd/                    # Application entry points
│   └── server/            # Server command
├── pkg/                   # Core packages
│   ├── parser/           # Sitemap parsers
│   ├── extractor/        # Keyword extraction
│   ├── api/              # API client management  
│   ├── storage/          # Data persistence
│   └── monitor/          # Health monitoring
├── internal/             # Private application code
│   ├── config/          # Configuration management
│   ├── service/         # Business logic services
│   └── handler/         # HTTP handlers
├── config/              # Configuration files
├── docs/                # Documentation
└── scripts/             # Build and deployment scripts
```

## Code Standards

### Naming Conventions
- Variables: camelCase
- Constants: UPPER_SNAKE_CASE  
- Types: PascalCase
- Files: kebab-case

### Performance Requirements
- Target: 10,000+ URLs/minute processing
- Memory limit: < 500MB
- API QPS: 100+
- Response time: < 100ms (local processing)

### Security Requirements
- AES-256 encryption for sensitive data
- Input validation for all user inputs
- No sensitive information in logs
- Parameterized queries (when database is added)

## Technology Stack

### Core Dependencies (from PRD)
- **HTTP Library**: Fiber v2 (fasthttp-based)
- **Logging**: zerolog (high-performance structured logging)
- **Config**: Viper for configuration management
- **Concurrency**: Native goroutines and channels
- **Encryption**: crypto/aes from standard library

### HTTP Client Optimization
The project uses fasthttp for maximum performance:
- Connection pooling enabled
- Request/response object pooling
- Optimized for high-throughput scenarios

## Development Guidelines

### Concurrency Patterns
- Use worker pools for bounded parallelism
- Implement pipeline patterns for data processing
- Utilize channels for inter-goroutine communication
- Apply context for cancellation and timeouts

### Error Handling
- Implement exponential backoff retry logic
- Use circuit breaker pattern for external APIs
- Provide detailed error context
- Log errors with structured logging

### Testing Strategy
- Unit tests for all core functions
- Benchmark tests for performance-critical code
- Integration tests for API interactions
- Mock external dependencies

## Configuration Management

The system uses a hierarchical configuration approach:
- Environment variables override file config
- Support for multiple environments (dev/staging/prod)
- Hot reload capability for runtime config changes

## Monitoring and Observability

### Metrics Collection
- Request/response metrics
- Error rates and types
- Worker pool utilization
- API client health status

### Health Checks
- API endpoint health monitoring
- Database connectivity (when implemented)
- Worker pool status
- Memory and CPU usage

## Deployment Considerations

### GitHub Actions Integration
The project is designed to run in GitHub Actions for automated content monitoring with scheduled execution every 6 hours.

### Container Deployment
- Multi-stage Docker builds for optimized images
- Single binary deployment
- Minimal Alpine Linux base image
- Resource-constrained environments supported