# Permission Control System Documentation

This document outlines the architecture and usage of the refactored permission control system in the platform-go project. The system is designed to be flexible, reusable, and decoupled from business logic.

## Architecture

The permission system is implemented in `src/middleware/auth_middleware.go`. It uses a factory pattern via the `Auth` struct to generate Gin middleware handlers. The core concept relies on separating the "extraction of context" (getting the Group ID) from the "verification of permission" (checking roles).

### Core Components

1.  **Auth Struct**: A wrapper that holds repository references needed for permission checks.
2.  **GIDExtractor**: A function type signature `func(c *gin.Context, repos *repositories.Repos) (uint, error)`. It is responsible for retrieving the Group ID (GID) from the request context (either body or URL parameters).
3.  **Middleware Generators**: Methods on the `Auth` struct that return `gin.HandlerFunc`.

## Usage

### 1. Initialization

In your router setup (e.g., `src/routes/router.go`), initialize the Auth middleware wrapper:

```go
repos_instance := repositories.New()
authMiddleware := middleware.NewAuth(repos_instance)
```

### 2. Permission Levels

The system supports the following permission levels:

*   **Admin()**: Restricts access to Super Admins only.
*   **UserOrAdmin()**: Allows access if the authenticated user matches the target `id` in the URL parameter, or is a Super Admin.
*   **GroupMember(extractor)**: Allows access if the user is a member of the target group (Admin, Manager, or User role) or is a Super Admin.
*   **GroupAdmin(extractor)**: Restricts access to Group Admins/Managers or Super Admins.

### 3. Extractors

Extractors tell the middleware how to find the Group ID associated with the request.

#### FromPayload

Used when the Group ID is present in the request body (JSON/Form). The DTO must implement the `GIDGetter` interface.

**Signature**: `middleware.FromPayload(dtoInstance)`

**Example**:
```go
// DTO definition
type CreateProjectDTO struct {
    GID uint `form:"g_id"`
}
func (d CreateProjectDTO) GetGID() uint { return d.GID }

// Route definition
r.POST("/projects", 
    authMiddleware.GroupMember(middleware.FromPayload(dto.CreateProjectDTO{})), 
    handler.CreateProject,
)
```

#### FromIDParam

Used when the request targets a specific resource by ID in the URL (e.g., `/projects/:id`). It requires a lookup function to resolve the Resource ID to a Group ID.

**Signature**: `middleware.FromIDParam(lookupFunction)`

**Example**:
```go
// Route definition
r.GET("/projects/:id", 
    authMiddleware.GroupMember(middleware.FromIDParam(repos.Project.GetGroupIDByProjectID)), 
    handler.GetProject,
)
```

## Extending the System

### Adding New Permission Rules

If a new type of permission check is needed (e.g., "System Monitor Only"), add a new method to the `Auth` struct in `src/middleware/auth_middleware.go`.

### Adding New Extraction Logic

If a GID needs to be extracted from a new source (e.g., a custom header), define a new function that returns a `GIDExtractor`.

```go
func FromCustomHeader(headerName string) GIDExtractor {
    return func(c *gin.Context, repos *repositories.Repos) (uint, error) {
        // Logic to parse header and return GID
    }
}
```

## Best Practices

1.  **Keep Handlers Clean**: Handlers should not perform permission checks. They should assume that if the request reaches them, the user is authorized.
2.  **DTO Interfaces**: Ensure all DTOs used with `FromPayload` implement `GetGID()`.
3.  **Lookup Efficiency**: Ensure lookup functions used in `FromIDParam` are efficient, as they run on every request to that endpoint.
