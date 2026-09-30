# Database Connection & Tooling

This document defines how the backend connects to PostgreSQL and how PostgreSQL is managed during local development.

The design covers:

- PostgreSQL connection management
- Connection pooling
- Startup connectivity validation
- Database configuration
- Local database tooling
- Database migration tooling

Schema design and persistence/repository implementation are covered separately.

---

## 2. Database Driver and Connection Pool

The backend uses `github.com/jackc/pgx/v5` with `pgxpool` for PostgreSQL connectivity.

`pgx` is a PostgreSQL-native Go driver and toolkit. `pgxpool` provides connection pooling on top of `pgx`.

The application uses the pool rather than creating a new PostgreSQL connection for every request.

### Decision

Using:

```
Go application
      |
      v
    pgx/v5
      |
      v
   pgxpool
      |
      v
  PostgreSQL
```

### Rationale

`pgx` was selected instead of using the generic `database/sql` abstraction because the application is specifically targeting PostgreSQL and can use `pgx`'s native PostgreSQL functionality directly.

`database/sql` remains a valid alternative. It provides a standard Go database interface that can work with different database drivers. `pgx` also provides an adapter for `database/sql` when that abstraction is required.

For this project, the application uses `pgx` directly.

---

## 3. Connection Pool Configuration

The connection pool is configured centrally in `internal/database`.

Current defaults:

| Setting             | Value | Purpose                                     |
| ------------------- | ----: | ------------------------------------------- |
| `MaxConns`          |  `25` | Maximum number of connections in the pool   |
| `MinConns`          |   `2` | Minimum number of maintained connections    |
| `MaxConnLifetime`   |  `1h` | Maximum lifetime of a connection            |
| `MaxConnIdleTime`   | `15m` | Maximum time an idle connection is retained |
| `HealthCheckPeriod` |  `1m` | Periodic pool health checking               |

These are project-level defaults for the MVP and can be adjusted as the application's workload and deployment environment become clearer.

The database URL remains part of the application configuration rather than being hard-coded in the database package.

---

## 4. Startup Connection and Fail-Fast Behaviour

Database connectivity is established during application startup.

The startup sequence is:

```
Load environment
      |
      v
config.Load()
      |
      v
Validate CFT_DB_URL
      |
      v
Create 5-second startup context
      |
      v
database.Connect()
      |
      v
Parse PostgreSQL connection configuration
      |
      v
Create pgxpool
      |
      v
pool.Ping()
      |
   +--+--+
   |     |
 success failure
   |     |
   v     v
start   return error
        and exit
```

`pgxpool.NewWithConfig` creates the pool but does not by itself guarantee that PostgreSQL is reachable.

Therefore, `database.Connect` explicitly calls `pool.Ping(ctx)`.

A five-second startup timeout prevents the application from waiting indefinitely when PostgreSQL is unavailable.

If the ping fails:

1. The pool is closed.
2. The connection error is returned.
3. Application startup fails.

This provides fail-fast behaviour for invalid database configuration or an unavailable PostgreSQL instance.

---

## 5. Application Lifecycle

The composition root in `cmd/api/main.go` owns the database pool lifecycle.

The relevant flow is:

```
config.Load()
    |
    v
database.Connect()
    |
    v
*pgxpool.Pool
    |
    +--> application components
    |
    v
pool.Close() on shutdown
```

The pool is created once during startup and is intended to be shared by the components that need database access.

`defer pool.Close()` ensures the pool is closed when the application exits.

Database credentials are not included in startup logs. Only non-sensitive connection information such as database name and host is logged.

---

## 6. Configuration

The database connection string is supplied through:

```
CFT_DB_URL
```

The existing configuration layer is responsible for:

- reading the environment variable
- validating that it is present
- constructing the application's typed configuration

The database package is responsible for:

- parsing the connection string
- configuring the pool
- establishing connectivity

This keeps application configuration separate from database connection management.

---

## 7. Local Database Tooling

A project-level `Makefile` provides common commands for local PostgreSQL operations.

### Connectivity

```
make db-ping
```

Runs:

```sql
SELECT 1;
```

through `psql` using `CFT_DB_URL`.

```
make db-shell
```

Opens an interactive `psql` session using the application's configured database URL.

```
make db-status
```

Checks the local PostgreSQL systemd service.

### Application

```
make run
```

Runs the API application.

```
make test
```

Runs the complete Go test suite.

---

## 8. Migration Tooling

Database schema changes are managed using `goose`.

The migration directory is:

```
migrations/
```

The Makefile provides:

```
make migrate-install
make migrate-status
make migrate-up
make migrate-down
make migrate-create NAME=<name>
```

The intended workflow is:

```
Create migration
      |
      v
Write SQL schema change
      |
      v
Apply migration with goose
      |
      v
PostgreSQL schema changes
```

Migrations are versioned changes to the database schema. The migration files provide a reproducible history of how the database structure evolves.

Initial schema design and migration contents will be implemented as part of the database schema task.

---

## 9. Testing

The database package contains tests for both configuration/connection failures and successful integration with PostgreSQL.

### Invalid DSN

An invalid PostgreSQL connection string must return an error.

### Unreachable PostgreSQL Host

A connection attempt to an unreachable host must fail within the supplied context timeout.

### Integration Connection

When `CFT_DB_URL` is available, the integration test:

1. Creates a connection pool.
2. Pings PostgreSQL.
3. Executes `SELECT 1`.
4. Verifies that the returned value is `1`.

The integration test is skipped when `CFT_DB_URL` is not configured, allowing the regular test suite to run without requiring a local PostgreSQL instance.

---

## 10. Alternatives Considered

### `database/sql` + PostgreSQL Driver

Go's `database/sql` package provides a generic database API. An application can use it with a PostgreSQL driver.

This provides database-driver abstraction and uses a standard Go interface, but the project does not currently require database portability.

Since the system is specifically built around PostgreSQL, direct `pgx` usage was chosen.

### `pgx/v5` + `pgxpool`

Chosen because it provides:

- PostgreSQL-native functionality
- connection pooling
- context-aware operations
- PostgreSQL-specific type support
- a direct API without requiring the `database/sql` abstraction

---

## 11. Project Responsibilities

| Component           | Responsibility                                      |
| ------------------- | --------------------------------------------------- |
| `internal/config`   | Load and validate application configuration         |
| `internal/database` | Configure and create the PostgreSQL connection pool |
| `cmd/api`           | Initialise and own the pool lifecycle               |
| `Makefile`          | Provide local database/application commands         |
| `goose`             | Apply and manage versioned database migrations      |
| `migrations/`       | Store versioned SQL schema changes                  |

---

## 12. Resulting Design

The resulting backend foundation is:

```
.env
  |
  v
godotenv
  |
  v
Process Environment
  |
  v
config.Load()
  |
  v
Validated Config
  |
  v
database.Connect()
  |
  v
pgxpool
  |
  v
PostgreSQL
```

Local development additionally uses:

```
Makefile
  |
  +--> psql
  |
  +--> goose
  |
  +--> go run
  |
  +--> go test
```

The connection layer is therefore established independently of the application’s persistence/repository layer. The next database task can build the schema and migrations on top of this foundation.
