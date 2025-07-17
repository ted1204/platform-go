# Platform API Project Documentation

## Introduction
This project is a RESTful API built with Go using the Gin framework. It integrates PostgreSQL for database management, supports user authentication and management, group and project resource control, JWT-based security, and MinIO for YAML storage.

## Tech Stack
- Go 1.21+
- Gin Web Framework
- PostgreSQL 15+
- GORM ORM
- JWT (JSON Web Tokens)
- MinIO (S3-compatible object storage)
- Swagger (Auto-generated API documentation)

## Project Structure

```
.
├── main.go
├── go.mod / go.sum
├── config/           # Configuration loader
├── models/           # Database models
├── dto/              # Request/Response data structures
├── handlers/         # Route handlers
├── middleware/       # JWT and other middlewares
├── routes/           # Route registration
└── docs/             # Swagger docs
```

## Installation & Execution

### Install dependencies
```bash
go mod download
```

### Run the service
```bash
go run main.go
```

### REST API doc
```bash
http://localhost:8080/swagger.html
```
