# Notely

A RESTful notes API written in Go, deployed to Google Cloud Run through a fully automated CI/CD pipeline using GitHub Actions. The pipeline enforces code quality gates on every pull request and handles build, migration, and deployment on every merge to main.

## Table of Contents

- [Overview](#overview)
- [API Reference](#api-reference)
- [CI/CD Pipeline](#cicd-pipeline)
  - [Continuous Integration](#continuous-integration)
  - [Continuous Deployment](#continuous-deployment)
- [Project Structure](#project-structure)
- [Local Development](#local-development)
- [Environment Variables](#environment-variables)
- [Database](#database)
- [Tech Stack](#tech-stack)

---

## Overview

Notely is a JSON API that lets authenticated users create and retrieve notes. Authentication is handled via API keys issued at registration. The service is stateless and backed by a Turso (libSQL) database.

The project was built with an emphasis on production-grade delivery practices: no code reaches the main branch without passing automated tests, security scans, and style checks, and every merge to main triggers an automatic deployment to Cloud Run.

---

## API Reference

All endpoints are mounted under `/v1`.

### Health Check

```
GET /v1/healthz
```

Returns `200 OK` if the server is running. No authentication required.

---

### Users

#### Create a user

```
POST /v1/users
```

**Request body:**

```json
{
  "name": "Alice"
}
```

**Response:** Returns the created user object along with a generated API key. Store the API key; it is required for all authenticated requests.

---

#### Get current user

```
GET /v1/users
```

**Headers:**

```
Authorization: ApiKey <your-api-key>
```

**Response:** Returns the authenticated user object.

---

### Notes

All notes endpoints require an `Authorization: ApiKey <your-api-key>` header.

#### Create a note

```
POST /v1/notes
```

**Request body:**

```json
{
  "note": "Your note text here"
}
```

**Response:** Returns the created note.

---

#### List notes

```
GET /v1/notes
```

**Response:** Returns an array of all notes belonging to the authenticated user.

---

## CI/CD Pipeline

The pipeline consists of two independent GitHub Actions workflows. Together they enforce a strict quality gate before production and automate the entire deployment process.

### Continuous Integration

**Workflow file:** [`.github/workflows/ci.yml`](.github/workflows/ci.yml)

**Triggered by:** Pull requests targeting `main`

The CI workflow runs two parallel jobs:

#### `tests` job

| Step | Tool | Purpose |
|------|------|---------|
| Unit Testing | `go test -cover ./...` | Runs all tests and reports coverage |
| Security Scan | `gosec` | Static analysis for common Go security vulnerabilities (e.g., SQL injection, hardcoded credentials, unsafe use of `crypto/rand`) |

#### `style` job

| Step | Tool | Purpose |
|------|------|---------|
| Formatting | `go fmt` | Fails if any file is not properly formatted |
| Static Analysis | `staticcheck` | Catches bugs, unused code, and API misuse that the compiler does not catch |

No pull request can be merged until both jobs pass. This prevents regressions, security issues, and inconsistent code style from reaching production.

---

### Continuous Deployment

**Workflow file:** [`.github/workflows/cd.yml`](.github/workflows/cd.yml)

**Triggered by:** Pushes to `main` (excluding Markdown file changes)

The CD workflow runs a single `deploy` job that executes the following steps in order:

```
Checkout -> Build Binary -> Authenticate to GCP -> Build & Push Docker Image -> Run Migrations -> Deploy to Cloud Run
```

#### Step-by-step breakdown

**1. Build the Go binary**

```bash
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o notely
```

Compiles a statically linked binary targeting `linux/amd64`. CGO is disabled to ensure the binary runs without any system library dependencies inside the minimal Docker image.

**2. Authenticate to Google Cloud**

Uses the `google-github-actions/auth` action with a service account credentials JSON stored as a GitHub Actions secret (`GCP_CREDENTIALS`). This grants the workflow permission to push images and deploy services without storing long-lived credentials in the repository.

**3. Build and push Docker image**

```bash
gcloud builds submit --tag us-central1-docker.pkg.dev/notely-503611/notely-ar-repo/notely:latest .
```

Uses Google Cloud Build to build the Docker image from the [Dockerfile](./Dockerfile) and push it to Artifact Registry. The image is based on `debian:stable-slim` with only `ca-certificates` added on top of the compiled binary, keeping the image minimal.

**4. Run database migrations**

```bash
goose turso $DATABASE_URL up
```

Runs any pending schema migrations against the Turso database before the new version of the service goes live. This ensures the database schema is always in sync with the application code at the point of deployment. The `DATABASE_URL` is injected from a GitHub Actions secret.

**5. Deploy to Cloud Run**

```bash
gcloud run deploy notely \
  --image us-central1-docker.pkg.dev/notely-503611/notely-ar-repo/notely:latest \
  --region us-central1 \
  --allow-unauthenticated \
  --max-instances=4
```

Deploys the new image to Cloud Run. The `--allow-unauthenticated` flag allows public access to the HTTP endpoints (authentication is handled at the application level via API keys). The `--max-instances=4` cap prevents runaway scaling costs.

#### Secrets required

| Secret | Description |
|--------|-------------|
| `GCP_CREDENTIALS` | Service account JSON with Cloud Build, Artifact Registry, and Cloud Run permissions |
| `DATABASE_URL` | Turso database connection string including auth token |

---

## Project Structure

```
.
├── .github/
│   └── workflows/
│       ├── ci.yml          # Pull request quality gates
│       └── cd.yml          # Automated deployment to Cloud Run
├── internal/
│   ├── auth/               # API key extraction from request headers
│   └── database/           # sqlc-generated database query functions
├── sql/
│   ├── schema/             # Goose migration files
│   │   ├── 001_users.sql
│   │   └── 002_notes.sql
│   └── queries/            # SQL queries used by sqlc
│       ├── users.sql
│       └── notes.sql
├── scripts/
│   ├── buildprod.sh        # Compiles a static Linux binary
│   └── migrateup.sh        # Runs goose migrations
├── Dockerfile              # Minimal debian image for the compiled binary
├── sqlc.yaml               # sqlc code generation config
├── main.go                 # Server setup, routing, database connection
├── middleware_auth.go       # API key authentication middleware
├── handler_user.go         # User creation and retrieval handlers
├── handler_notes.go        # Note creation and retrieval handlers
├── handler_ready.go        # Health check handler
├── models.go               # Response model conversions
└── json.go                 # JSON response helpers
```

---

## Local Development

**Prerequisites:**

- Go 1.22+
- A Turso database (or any libSQL-compatible endpoint)
- `goose` for running migrations: `go install github.com/pressly/goose/v3/cmd/goose@latest`
- `sqlc` if you intend to modify queries: `go install github.com/sqlc-dev/sqlc/cmd/sqlc@latest`

**1. Clone the repository**

```bash
git clone https://github.com/Eng-Moaz/notely.git
cd notely
```

**2. Set up your environment**

Copy the example below into a `.env` file at the project root:

```env
PORT=8080
DATABASE_URL=libsql://<your-database>.turso.io?authToken=<your-auth-token>
```

**3. Run migrations**

```bash
./scripts/migrateup.sh
```

**4. Start the server**

```bash
go run .
```

The server will be available at `http://localhost:8080`.

---

## Environment Variables

| Variable | Required | Description |
|----------|----------|-------------|
| `PORT` | Yes | Port the HTTP server listens on |
| `DATABASE_URL` | No | libSQL connection string. If not set, the server starts without database endpoints |

---

## Database

Schema is managed with [Goose](https://github.com/pressly/goose) and SQL queries are type-safe via [sqlc](https://sqlc.dev).

**Schema migrations** live in `sql/schema/` and are numbered sequentially. Run `./scripts/migrateup.sh` to apply all pending migrations.

**Query generation:** After modifying any `.sql` file in `sql/queries/`, run `sqlc generate` to regenerate the Go database layer in `internal/database/`.

---

## Tech Stack

| Component | Technology |
|-----------|------------|
| Language | Go 1.22 |
| HTTP Router | [chi](https://github.com/go-chi/chi) |
| Database | [Turso](https://turso.tech) (libSQL) |
| ORM / Query Layer | [sqlc](https://sqlc.dev) |
| Migrations | [Goose](https://github.com/pressly/goose) |
| Containerization | Docker |
| Container Registry | Google Artifact Registry |
| Hosting | Google Cloud Run |
| CI/CD | GitHub Actions |
| Security Scanning | [gosec](https://github.com/securego/gosec) |
| Static Analysis | [staticcheck](https://staticcheck.dev) |
