# Integrations

This directory contains self-contained integration packages for automatically instrumenting Go libraries with New Relic.

## Available Integrations

| Integration | Library | Lines | Examples |
|------------|---------|-------|----------|
| [nragent](./nragent/) | New Relic Agent | 1,412 | Core initialization |
| [nrnethttp](./nrnethttp/) | net/http (stdlib) | 3,009 | 2 examples |
| [nrgrpc](./nrgrpc/) | gRPC | 1,430 | 1 example |
| [nrgin](./nrgin/) | Gin Web Framework | 745 | 23 examples |
| [nrmysql](./nrmysql/) | MySQL/database/sql | 1,071 | 1 example |
| [nrgochi](./nrgochi/) | Go-chi Router | 642 | 1 example |
| [nrslog](./nrslog/) | Structured Logging (slog) | 150 | 1 example |

## Directory Structure

Each integration follows a consistent structure:

```
integrations/nr{library}/
├── {library}.go        # Parsing/detection logic
├── codegen.go          # DST node generation for instrumentation
├── parsing_test.go     # Tests for parsing logic
├── codegen_test.go     # Tests for code generation
└── example/            # End-to-end example applications
    ├── {example1}/
    │   ├── main.go
    │   └── expect.ref  # Expected instrumentation output
    └── go.mod          # Shared dependencies for examples
```

## Quick Start

### Using an Integration

All integrations are automatically detected and applied when you run:

```bash
go run . instrument <path-to-your-app>
```

The tool will:
1. Detect which libraries you use (Gin, gRPC, MySQL, etc.)
2. Apply the corresponding integrations automatically
3. Generate a `.diff` file with instrumentation for review

### Creating a New Integration

📖 **See [GUIDE.md](./GUIDE.md) for a complete tutorial on creating integrations**

Quick overview:

1. **Create the package structure:**
   ```bash
   mkdir -p integrations/nr{library}/example
   touch integrations/nr{library}/{library}.go
   touch integrations/nr{library}/codegen.go
   ```

2. **Implement detection logic** (`{library}.go`)
3. **Implement code generation** (`codegen.go`)
4. **Write tests** (`*_test.go`)
5. **Register in `parser/manager.go`**
6. **Add end-to-end examples**

See [GUIDE.md](./GUIDE.md) for detailed step-by-step instructions.

## Integration Types

### By Complexity

**Simple** (100-700 lines)
- `nrslog` - Logging instrumentation
- `nrgochi` - Router middleware
- `nrgin` - Web framework middleware

**Medium** (700-1,500 lines)
- `nrmysql` - Database instrumentation
- `nragent` - Agent initialization
- `nrgrpc` - gRPC interceptors

**Complex** (1,500+ lines)
- `nrnethttp` - HTTP client/server instrumentation (foundational)

### By Function Type

**Middleware/Interceptor Pattern**
- `nrgin`, `nrgochi` - Add middleware to routers
- `nrgrpc` - Add interceptors to clients/servers

**Wrapper Pattern**
- `nrslog` - Wrap logging handlers
- `nrnethttp` - Wrap HTTP handlers

**Context Enhancement**
- `nrmysql` - Add context to database calls
- `nrnethttp` - Add transaction to request context

**Initialization**
- `nragent` - Initialize New Relic application

## Testing

### Run All Integration Tests

```bash
# Unit tests
go test ./integrations/...

# Validation tests
./validation-tests/testrunner
```

### Run Specific Integration Tests

```bash
# Test a specific integration
go test ./integrations/nrgin/...

# Test a specific example
go run . instrument integrations/nrgin/example/basic
```

## Contributing

1. Read [GUIDE.md](./GUIDE.md) for development guidelines
2. Follow the structure of existing integrations
3. Ensure all tests pass before submitting
4. Add at least one end-to-end example

## Design Principles

1. **Self-Contained**: Each integration is independent and fully contained
2. **Non-Invasive**: Static analysis only - no code execution
3. **Reviewable**: Generate `.diff` files for human review
4. **Tested**: Unit tests + end-to-end tests for each integration
5. **Documented**: Clear examples showing instrumentation results

## Architecture

```
┌─────────────────────────────────────────────────────────┐
│                     CLI (cmd/)                          │
└─────────────────────┬───────────────────────────────────┘
                      │
┌─────────────────────▼───────────────────────────────────┐
│            Instrumentation Manager (parser/)            │
│  - Loads packages with DST                             │
│  - Orchestrates instrumentation passes                 │
│  - Manages tracing state                               │
└─────────────────────┬───────────────────────────────────┘
                      │
        ┌─────────────┴──────────────┬──────────────┬─────────────┐
        │                            │              │             │
┌───────▼────────┐          ┌────────▼──────┐  ┌───▼────────┐ ┌─▼─────────┐
│  nrgin/        │          │  nrgrpc/      │  │ nrmysql/   │ │ nrslog/   │
│  - gin.go      │          │  - grpc.go    │  │ - mysql.go │ │ - slog.go │
│  - codegen.go  │   ...    │  - codegen.go │  │ - codegen.go│ │ - codegen.go│
│  - tests       │          │  - tests      │  │ - tests    │ │ - tests   │
│  - examples    │          │  - examples   │  │ - examples │ │ - examples│
└────────────────┘          └───────────────┘  └────────────┘ └───────────┘
```

## Learn More

- **Integration Development**: [GUIDE.md](./GUIDE.md)
- **Project Architecture**: `memory/architecture.md`
- **Main README**: [../README.md](../README.md)
