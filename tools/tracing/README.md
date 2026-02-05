# Tracing Package

A Zipkin-compatible distributed tracing implementation for Go applications. This package provides a lightweight tracer that can generate Zipkin-format spans for monitoring and debugging distributed systems.

## Features

- **Hierarchical Span Tracking**: Automatically maintains parent-child relationships between spans
- **Thread-Safe Operations**: Safe for concurrent use with proper synchronization
- **Local File Export**: Save traces to JSON files for offline analysis
- **Zipkin Server Integration**: Real-time span submission to Zipkin servers
- **Automatic Batching**: Efficient batching of spans with configurable batch size and intervals
- **Retry Logic**: Built-in retry mechanism with exponential backoff for server failures
- **Singleton Pattern**: Global tracer instance available throughout your application

## Installation

```go
import "github.com/klever-io/klever-go/tools/tracing"
```

## Environment Configuration

The tracing system can be automatically configured using environment variables. This is the recommended approach for production deployments.

### Environment Variables

| Variable | Description | Default | Example |
|----------|-------------|---------|---------|
| `KLEVER_TRACING_ENABLED` | Enable/disable tracing | `false` | `true` |
| `KLEVER_TRACING_SERVER_URL` | Zipkin server URL | `""` | `http://localhost:9411` |
| `KLEVER_TRACING_BATCH_SIZE` | Spans per batch | `100` | `50` |
| `KLEVER_TRACING_PUSH_INTERVAL` | Batch push interval | `5s` | `10s` |
| `KLEVER_TRACING_SERVICE_NAME` | Service identifier | `KleverGo` | `validator-01` |
| `KLEVER_TRACING_SAVE_ON_EXIT` | Save traces on shutdown | `false` | `true` |
| `KLEVER_TRACING_SAVE_PATH` | Directory for saved traces | `./traces` | `/var/log/traces` |

### Automatic Initialization

Add this to your main function:

```go
func main() {
    // Initialize tracing from environment variables
    tracing.MustInitialize()
    defer tracing.Shutdown()
    
    // Your application code...
    // The singleton tracer is now configured globally
}
```

### Docker Example

```yaml
services:
  klever-node:
    image: kleverapp/klever-go:latest
    environment:
      - KLEVER_TRACING_ENABLED=true
      - KLEVER_TRACING_SERVER_URL=http://zipkin:9411
      - KLEVER_TRACING_SERVICE_NAME=node-1
```

### Multi-Instance Support

When running multiple instances of KleverGo on the same server or pointing to the same Zipkin server, each instance needs a unique service name to distinguish traces. The tracing system provides several ways to ensure uniqueness:

#### Automatic Service Name Generation

If no `KLEVER_TRACING_SERVICE_NAME` is provided, the system automatically generates a unique name using:
- Hostname + Process ID (e.g., `KleverGo-server1-12345`)
- NODE_NAME environment variable if available (common in Kubernetes)
- INSTANCE_ID environment variable if available

#### Manual Configuration

You can explicitly set unique service names for each instance:

```bash
# Instance 1
KLEVER_TRACING_SERVICE_NAME=klever-validator-1 ./klever-node

# Instance 2  
KLEVER_TRACING_SERVICE_NAME=klever-validator-2 ./klever-node

# Instance 3
KLEVER_TRACING_SERVICE_NAME=klever-observer-1 ./klever-node
```

#### Kubernetes/Container Orchestration

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: klever-validators
spec:
  replicas: 3
  template:
    spec:
      containers:
      - name: klever-node
        env:
        - name: INSTANCE_ID
          valueFrom:
            fieldRef:
              fieldPath: metadata.name
        - name: NODE_NAME
          valueFrom:
            fieldRef:
              fieldPath: spec.nodeName
        - name: KLEVER_TRACING_ENABLED
          value: "true"
        - name: KLEVER_TRACING_SERVER_URL
          value: "http://zipkin-service:9411"
```

#### IP Address Tracking

Each span automatically includes the local IP address of the instance in the endpoint information, making it easier to correlate traces with specific machines in your infrastructure.

## Quick Start

### Simplified API (Recommended)

The package provides a simplified API that handles all the boilerplate:

```go
package main

import (
    "time"
    "github.com/klever-io/klever-go/tools/tracing"
)

func main() {
    // Initialize tracing from environment variables
    tracing.MustInitialize()
    defer tracing.Shutdown()
    
    // Use the defer pattern for automatic span lifecycle
    defer tracing.StartSpan(
        "main.operation",
        "version", "1.0",
        "user", "alice",
    )()
    
    doWork()
}

func doWork() {
    // Child spans are automatically nested
    defer tracing.StartSpan("work.process")()
    
    // Add tags to the current span
    tracing.AddTag("items.count", "42")
    
    // Add annotations for important events
    tracing.AddAnnotation("Starting processing")
    
    time.Sleep(100 * time.Millisecond)
    
    tracing.AddAnnotationf("Processed %d items", 42)
}
```

### Basic Usage (Direct Tracer Access)

```go
package main

import (
    "fmt"
    "time"
    "github.com/klever-io/klever-go/tools/tracing"
)

func main() {
    // Get the singleton tracer instance
    tracer := tracing.GetZipkinTracer()
    
    // Start a root span
    tracer.Start("main-operation")
    
    // Do some work
    doWork()
    
    // Stop the span
    tracer.Stop("main-operation")
    
    // Get measurement
    duration := tracer.GetMeasurement("main-operation")
    fmt.Printf("Operation took: %v\n", duration)
    
    // Save traces to file
    filename, err := tracer.SaveSpansLog("my-trace")
    if err == nil {
        fmt.Printf("Traces saved to: %s\n", filename)
    }
}

func doWork() {
    tracer := tracing.GetZipkinTracer()
    
    // Create a child span
    tracer.Start("sub-operation")
    time.Sleep(100 * time.Millisecond)
    tracer.Stop("sub-operation")
}
```

### With Zipkin Server

```go
package main

import (
    "time"
    "github.com/klever-io/klever-go/tools/tracing"
)

func main() {
    // Create a tracer with server push enabled
    tracer := tracing.NewZipkinTracerWithServer(
        "http://localhost:9411",  // Zipkin server URL
        100,                       // Batch size
        5*time.Second,            // Push interval
    )
    defer tracer.DisableServerPush()
    
    // Use the tracer
    tracer.Start("operation")
    // ... do work ...
    tracer.Stop("operation")
    
    // Force immediate flush of pending spans
    tracer.Flush()
}
```

### Dynamic Server Configuration

```go
// Start with local-only tracing
tracer := tracing.GetZipkinTracer()

// Later, enable server push
tracer.EnableServerPush(
    "http://localhost:9411",
    50,
    10*time.Second,
)

// When done, disable server push
tracer.DisableServerPush()
```

## API Reference

### Simplified API Functions (Recommended)

#### `StartSpan(name string, tagsKV ...string) func()`
Starts a new span with optional tags. Returns a function that stops the span when called.
Designed for use with the defer pattern.

```go
defer tracing.StartSpan("operation.name", "key1", "value1", "key2", "value2")()
```

#### `StartSpanWithTags(name string, tags map[string]string) func()`
Starts a new span with tags provided as a map.

```go
tags := map[string]string{"user": "alice", "action": "create"}
defer tracing.StartSpanWithTags("user.action", tags)()
```

#### `StartSpanf(format string, args ...any) func()`
Starts a span with a formatted name.

```go
defer tracing.StartSpanf("process.item.%d", itemID)()
```

#### `AddTag(key, value string)`
Adds a tag to the currently active span.

```go
tracing.AddTag("status", "success")
```

#### `AddTagf(key, format string, args ...any)`
Adds a formatted tag to the currently active span.

```go
tracing.AddTagf("count", "Processed %d items", count)
```

#### `AddAnnotation(value string)`
Adds an annotation (event marker) to the currently active span.

```go
tracing.AddAnnotation("Cache miss occurred")
```

#### `AddAnnotationf(format string, args ...any)`
Adds a formatted annotation to the currently active span.

```go
tracing.AddAnnotationf("Retry attempt %d of %d", attempt, maxRetries)
```

#### `TraceIf(fn func(*ZipkinTracer))`
Executes the provided function only if tracing is enabled.

```go
tracing.TraceIf(func(tracer *ZipkinTracer) {
    // Expensive tracing operations
    tracer.AddAnnotation(expensiveComputation())
})
```

### Core Functions

#### `NewZipkinTracer() *ZipkinTracer`
Creates a new tracer instance for local tracing only.

#### `NewZipkinTracerWithServer(serverURL string, batchSize int, pushInterval time.Duration) *ZipkinTracer`
Creates a new tracer instance with Zipkin server integration.

#### `GetZipkinTracer() *ZipkinTracer`
Returns the singleton tracer instance (thread-safe).

#### `ResetZipkinTracer()`
Resets the singleton instance, creating a fresh tracer.

### Span Operations

#### `Start(identifier string)`
Starts a new span with the given identifier. If there's an active span, the new span becomes its child.

#### `Stop(identifier string)`
Stops the span with the given identifier and sets its duration. Returns to the parent span if applicable.

#### `GetMeasurement(identifier string) time.Duration`
Returns the duration of a completed span by its identifier.

### Export and Server Push

#### `SaveSpansLog(name string) (string, error)`
Exports all spans to a JSON file. Returns the filename and any error.

#### `EnableServerPush(serverURL string, batchSize int, pushInterval time.Duration)`
Enables pushing spans to a Zipkin server.

#### `DisableServerPush()`
Stops pushing spans to the server and performs final flush.

#### `Flush()`
Immediately sends all pending spans to the server.

## Advanced Usage

### Nested Spans Example (Simplified API)

```go
func processOrder(orderID string) {
    // Parent span with tags
    defer tracing.StartSpan(
        "order.process",
        "order.id", orderID,
        "order.type", "standard",
    )()
    
    // Child spans are automatically nested
    validateOrder(orderID)
    processPayment(orderID)
    shipOrder(orderID)
}

func validateOrder(orderID string) {
    defer tracing.StartSpan("order.validate", "order.id", orderID)()
    
    // Add runtime information
    tracing.AddAnnotation("Starting validation")
    // ... validation logic ...
    tracing.AddTag("validation.result", "success")
}

func processPayment(orderID string) {
    defer tracing.StartSpan("payment.process", "order.id", orderID)()
    
    // ... payment logic ...
    tracing.AddTagf("payment.amount", "%.2f", 99.99)
}

func shipOrder(orderID string) {
    defer tracing.StartSpan("order.ship", "order.id", orderID)()
    
    // ... shipping logic ...
    tracing.AddAnnotationf("Shipped via %s", "FedEx")
}
```

### Nested Spans Example (Direct Tracer)

```go
func processOrder(orderID string) {
    tracer := tracing.GetZipkinTracer()
    
    tracer.Start("process-order")
    defer tracer.Stop("process-order")
    
    // Validate order
    tracer.Start("validate-order")
    validateOrder(orderID)
    tracer.Stop("validate-order")
    
    // Process payment
    tracer.Start("process-payment")
    processPayment(orderID)
    tracer.Stop("process-payment")
    
    // Ship order
    tracer.Start("ship-order")
    shipOrder(orderID)
    tracer.Stop("ship-order")
}
```

### Integration with HTTP Middleware

```go
func TracingMiddleware(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        // Simplified API with automatic span management
        defer tracing.StartSpan(
            fmt.Sprintf("http.%s", strings.ToLower(r.Method)),
            "http.method", r.Method,
            "http.path", r.URL.Path,
            "http.url", r.URL.String(),
            "user.agent", r.UserAgent(),
        )()
        
        // Add request ID if present
        if reqID := r.Header.Get("X-Request-ID"); reqID != "" {
            tracing.AddTag("request.id", reqID)
        }
        
        next.ServeHTTP(w, r)
    })
}
```

### Error Handling with Tracing

```go
func riskyOperation() (err error) {
    defer tracing.StartSpan("operation.risky")()
    
    // Capture panics and errors in traces
    defer func() {
        if r := recover(); r != nil {
            tracing.AddTag("error", "panic")
            tracing.AddAnnotationf("Panic: %v", r)
            
            // Save traces for debugging
            if tracer := tracing.GetConfiguredTracer(); tracer != nil {
                tracer.SaveSpansLog("panic-trace")
            }
            panic(r)
        } else if err != nil {
            tracing.AddTag("error", "true")
            tracing.AddTag("error.message", err.Error())
        }
    }()
    
    // Risky code here
    tracing.AddAnnotation("Starting risky operation")
    
    // ... actual logic ...
    
    return nil
}
```

## Configuration

### Batch Size
Controls how many spans are accumulated before sending to the server. Larger batches are more efficient but may delay visibility.

### Push Interval
Maximum time to wait before sending a partial batch. Ensures spans are sent even with low traffic.

### Retry Logic
The package includes automatic retry with exponential backoff:
- 3 retry attempts maximum
- Backoff intervals: 1s, 2s, 4s
- Only retries on server errors (5xx), not client errors (4xx)

## Testing

Run the test suite:

```bash
go test ./tools/tracing/...
```

Run with coverage:

```bash
go test -cover ./tools/tracing/...
```

Run specific tests:

```bash
go test -run TestServerPush ./tools/tracing/...
```

## Zipkin Server Setup

To use the server push features, you need a Zipkin server:

### Using Docker

```bash
docker run --name zipkin --rm -d -p 9411:9411 openzipkin/zipkin
```

### Using Docker Compose

```yaml
version: '3'
services:
  zipkin:
    image: openzipkin/zipkin
    ports:
      - "9411:9411"
    environment:
      - STORAGE_TYPE=mem
```

Access the Zipkin UI at http://localhost:9411

## Output Format

The package generates spans in Zipkin v2 JSON format:

```json
[
  {
    "traceId": "5a8b7c6d4e3f2a1b",
    "id": "1a2b3c4d5e6f7890",
    "parentId": "9f8e7d6c5b4a3210",
    "name": "operation-name",
    "timestamp": 1634567890123456,
    "duration": 1234567,
    "localEndpoint": {
      "serviceName": "KleverGo"
    },
    "kind": "SERVER"
  }
]
```

## Performance Considerations

- **Minimal Overhead**: When disabled, tracing adds only ~2-5ns overhead per call with zero allocations
- **Efficient When Enabled**: Span creation takes only ~650ns with 514B memory allocation
- **Singleton Pattern**: The global instance reduces memory overhead but assumes sequential span tracking within a goroutine
- **Concurrent Safety**: All operations are thread-safe, but concurrent goroutines should be aware of shared active span state
- **Memory Usage**: Spans are kept in memory until explicitly saved or pushed to server (with configurable limits)
- **Batching**: Reduces network overhead when using server push
- **File I/O**: SaveSpansLog performs synchronous file write; consider calling asynchronously for large traces

For detailed performance metrics, see [BENCHMARK.md](BENCHMARK.md)

## Limitations

- The singleton pattern's active span tracking assumes sequential execution within a single goroutine
- For true concurrent tracing across goroutines, consider using separate tracer instances or context-based tracing
- Maximum file name length for saved traces is 64 characters (automatically truncated)
- Server push requires network connectivity and may fail silently if the server is unreachable

## Contributing

When contributing to this package:

1. Maintain backward compatibility
2. Add tests for new features
3. Update this README for API changes
4. Follow Go best practices and conventions
5. Ensure thread safety for all public methods

## License

See the project's main LICENSE file.