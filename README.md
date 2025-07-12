# Go Service Framework

A lightweight, production-ready Go framework for building microservices with HTTP/gRPC servers, database connections, and comprehensive observability features.

## 🚀 Features

- **Multiple Server Support**: HTTP and gRPC servers with graceful shutdown
- **Database Integration**: PostgreSQL connection pooling with health checks
- **Observability**: Prometheus metrics, health checks, and readiness probes
- **Signal Handling**: Graceful shutdown on SIGINT/SIGTERM
- **Structured Logging**: JSON logging with zerolog
- **Performance Monitoring**: Built-in pprof endpoints
- **Modular Architecture**: Options pattern for flexible configuration

## 📦 Installation

```bash
go get github.com/jetbrainer/app
```

## 🏗️ Quick Start

### Basic HTTP Server

```go
package main

import (
    "context"
    "log"
    
    "github.com/jetbrainer/app"
)

func main() {
    ctx := context.Background()
    
    service, err := app.New(ctx, "my-service",
        app.WithTechHTTPServerOption(":8080"),
    )
    if err != nil {
        log.Fatal(err)
    }
    
    defer service.Stop()
    
    if err := service.Start(); err != nil {
        log.Fatal(err)
    }
}
```

### HTTP + gRPC + Database

```go
package main

import (
    "context"
    "log"
    
    "github.com/jackc/pgx/v5/pgxpool"
    "github.com/jetbrainer/app"
)

func main() {
    ctx := context.Background()
    
    // Database configuration
    dbConfig, _ := pgxpool.ParseConfig("postgres://user:password@localhost/mydb")
    
    service, err := app.New(ctx, "my-service",
        app.WithTechHTTPServerOption(":8080"),
        app.WithGRPCServer(":9090"),
        app.WithDB(*dbConfig),
    )
    if err != nil {
        log.Fatal(err)
    }
    
    defer service.Stop()
    
    if err := service.Start(); err != nil {
        log.Fatal(err)
    }
}
```

## 🔧 Configuration Options

### HTTP Server

```go
app.WithTechHTTPServerOption(":8080")
```

The technical HTTP server includes:
- **Health endpoints**: `/health/live`, `/health/ready`
- **Metrics endpoint**: `/metrics` (Prometheus format)
- **Debug endpoints**: `/debug/pprof/*` (Go profiling)

### gRPC Server

```go
app.WithGRPCServer(":9090")
```

Add gRPC services after initialization:

```go
service.AddGRPCService("my-server", myServiceImpl, &pb.MyService_ServiceDesc)
```

### Database

```go
dbConfig, _ := pgxpool.ParseConfig("postgres://user:password@localhost/mydb")
app.WithDB(*dbConfig)
```

Supports PostgreSQL with connection pooling via pgx/v5.

### Redis (Planned)

```go
app.WithRedis() // Implementation in progress
```

## 📊 Monitoring & Observability

### Health Checks

The framework provides two types of health checks:

**Liveness Probe** (`/health/live`):
- Checks if the application is running
- Returns 200 if all critical components are operational

**Readiness Probe** (`/health/ready`):
- Checks if the application is ready to serve traffic
- Returns 200 when all services are initialized and ready

### Metrics

Prometheus metrics are automatically exposed at `/metrics`:

- Go runtime metrics (memory, GC, goroutines)
- Process metrics (CPU, memory usage)
- Custom application metrics (can be added)

### Profiling

Debug endpoints available at `/debug/pprof/`:
- `/debug/pprof/` - Index page
- `/debug/pprof/goroutine` - Goroutine stack traces
- `/debug/pprof/heap` - Heap profile
- `/debug/pprof/profile` - CPU profile
- `/debug/pprof/trace` - Execution trace

## 🔄 Lifecycle Management

### Service Startup

```go
service.Start() // Blocks until shutdown signal
```

The startup process:
1. Starts all HTTP servers
2. Starts all gRPC servers
3. Initializes database connections
4. Performs readiness checks
5. Waits for shutdown signal

### Graceful Shutdown

The service automatically handles `SIGINT` and `SIGTERM`:

1. Stops accepting new connections
2. Waits for active requests to complete
3. Closes database connections
4. Stops all subservices
5. Exits gracefully

## 🏛️ Architecture

### Service Structure

```go
type Service struct {
    Name        string
    GRPCServers []*GRPCServer
    HTTPServers []*http.Server
    DB          *pgxpool.Pool
    SubServices map[string]SubService
    // internal fields...
}
```

### SubServices

Implement the `SubService` interface to add custom services:

```go
type SubService interface {
    Ready() bool
    Name() string
    Close() error
}
```

Example:

```go
type MySubService struct {
    name string
    ready bool
}

func (s *MySubService) Ready() bool { return s.ready }
func (s *MySubService) Name() string { return s.name }
func (s *MySubService) Close() error { return nil }

// Register with service
service.SubServices["my-service"] = &MySubService{name: "my-service", ready: true}
```

## 📝 Examples

### Custom HTTP Routes

```go
package main

import (
    "net/http"
    "github.com/go-chi/chi/v5"
    "github.com/jetbrainer/app"
)

func main() {
    ctx := context.Background()
    
    service, _ := app.New(ctx, "my-service",
        app.WithTechHTTPServerOption(":8080"),
    )
    
    // Add custom HTTP server
    router := chi.NewRouter()
    router.Get("/api/hello", func(w http.ResponseWriter, r *http.Request) {
        w.Write([]byte("Hello, World!"))
    })
    
    customServer := &http.Server{
        Addr:    ":8081",
        Handler: router,
    }
    
    service.AddHTTPServer(customServer)
    
    defer service.Stop()
    service.Start()
}
```

### Database Usage

```go
func handleRequest(service *app.Service) http.HandlerFunc {
    return func(w http.ResponseWriter, r *http.Request) {
        var count int
        err := service.DB.QueryRow(r.Context(), "SELECT COUNT(*) FROM users").Scan(&count)
        if err != nil {
            http.Error(w, err.Error(), http.StatusInternalServerError)
            return
        }
        
        fmt.Fprintf(w, "Users count: %d", count)
    }
}
```

## 🔍 Troubleshooting

### Common Issues

**Service not starting**:
- Check if ports are available
- Verify database connection string
- Check logs for initialization errors

**Health checks failing**:
- Verify database connectivity
- Check if all subservices are ready
- Review service dependencies

**High memory usage**:
- Use `/debug/pprof/heap` to analyze memory usage
- Check database connection pool settings
- Monitor goroutine count with `/debug/pprof/goroutine`

### Logging

The framework uses structured JSON logging:

```go
import "github.com/rs/zerolog/log"

log.Info().Str("component", "http-server").Msg("server started")
log.Error().Err(err).Msg("database connection failed")
```

## 🤝 Contributing

1. Fork the repository
2. Create a feature branch
3. Add tests for new functionality
4. Ensure all tests pass
5. Submit a pull request

## 📋 Requirements

- Go 1.23.7+
- PostgreSQL (optional, for database features)
- Redis (optional, planned feature)

## 🔗 Dependencies

- **HTTP Router**: `github.com/go-chi/chi/v5`
- **Database**: `github.com/jackc/pgx/v5`
- **Metrics**: `github.com/prometheus/client_golang`
- **Logging**: `github.com/rs/zerolog`
- **gRPC**: `google.golang.org/grpc`

## 📄 License

This project is licensed under the MIT License - see the LICENSE file for details.

## 🚧 Roadmap

- [ ] Redis integration
- [ ] OpenTelemetry tracing
- [ ] Configuration file support
- [ ] Rate limiting middleware
- [ ] Circuit breaker pattern
- [ ] Service discovery integration
- [ ] Docker containerization examples
- [ ] Kubernetes deployment manifests

## 💡 Best Practices

### Production Deployment

1. **Environment Variables**:
   ```bash
   export DB_URL="postgres://user:pass@localhost/db"
   export HTTP_PORT=":8080"
   export GRPC_PORT=":9090"
   ```

2. **Health Check Configuration**:
   ```yaml
   # Kubernetes example
   livenessProbe:
     httpGet:
       path: /health/live
       port: 8080
     initialDelaySeconds: 30
     periodSeconds: 10
   
   readinessProbe:
     httpGet:
       path: /health/ready
       port: 8080
     initialDelaySeconds: 5
     periodSeconds: 5
   ```

3. **Monitoring Setup**:
   ```yaml
   # Prometheus scrape config
   scrape_configs:
     - job_name: 'my-service'
       static_configs:
         - targets: ['localhost:8080']
       metrics_path: /metrics
   ```

### Development Tips

- Use the technical HTTP server for debugging and monitoring
- Implement proper error handling in your business logic
- Add custom metrics for business KPIs
- Use structured logging throughout your application
- Test graceful shutdown behavior
- Monitor resource usage with pprof endpoints

---

**Happy coding! 🎉**
