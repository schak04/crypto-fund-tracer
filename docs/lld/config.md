# Configuration Design

## Problem

The application uses environment variables for runtime configuration. A `.env`
file is used for local development, but it is only a plain-text file containing
`KEY=VALUE` pairs; it is not itself the process environment.

Go's `os.Getenv()` reads variables from the process environment and does not
parse `.env` files. Therefore, a mechanism is required to load local `.env`
configuration into the process environment before the application's
configuration is loaded.

---

## Considered Approaches

### 1. Custom `.env` Loader

Implementing a small loader in the `config` package to parse `.env` and populate
the process environment.

- **Pros:** No external dependency; complete control over loading behaviour.
- **Cons:** Introduces custom parsing and maintenance code for a problem already solved by the community.

### 2. `godotenv`

Using `github.com/joho/godotenv` to load `.env` during application startup.

- **Pros:** Minimal implementation effort, established `.env` parsing, and keeps `.env` loading separate from the application's configuration validation.
- **Cons:** Adds one external dependency.

### 3. External Development Tooling

Keeping the application dependent only on the process environment and use tooling such as `direnv` or Make to inject `.env` values.

- **Pros:** Keeps `.env` handling outside the application and maintains a strict environment-variable-based configuration boundary.
- **Cons:** Adds development-environment/tooling requirements.

---

## Decision

The project uses **`godotenv`** for local `.env` loading.

The additional dependency is small and focused, while avoiding custom parsing logic and additional tooling. `godotenv` is responsible only for bridging `.env` configuration into the process environment; `config.Load()` remains responsible for applying defaults, parsing values, validating configuration, and constructing the application's `Config` object.

The resulting configuration flow is:

```
.env
  |
  V
godotenv
  |
  V
Process Environment
  |
  V
config.Load()
  |
  V
Validated Config
```

Production and other managed environments can provide environment variables directly without requiring a `.env` file.

---
