# Lemwood Mirror Project Context

## Project Overview

**Lemwood Mirror** is a self-hosted mirroring service designed to automatically track and download the latest releases of specific software (primarily Minecraft launchers like FCL, ZL, ZL2) from GitHub. It provides a web interface for users to browse and download these mirrored versions, along with detailed usage statistics.

The project consists of:
- **Backend:** A Go application that handles GitHub scraping/API interaction, file downloading, scheduling, data storage (SQLite), and serving the HTTP API/Web UI.
- **Frontend:** A Vue 3 application (built with Vite and Vuetify) that provides a user-friendly interface for browsing versions and viewing statistics.

## Architecture & Technologies

### Backend (Go)
- **Entry Point:** `cmd/mirror/main.go`
- **Framework:** Standard `net/http` with `http.ServeMux`.
- **Key Libraries:**
    - `github.com/gocolly/colly/v2`: Web scraping to resolve GitHub repository URLs.
    - `github.com/google/go-github/v50`: Interaction with the GitHub API.
    - `github.com/robfig/cron/v3`: Scheduling periodic update checks.
    - `modernc.org/sqlite`: Embedded database for statistics.
- **Configuration:** `config.json` loaded via `internal/config`.
- **Storage:** Local filesystem for downloaded artifacts; SQLite for metadata/stats.

### Frontend (Vue 3)
- **Location:** `web/` directory.
- **Framework:** Vue 3 + Vite.
- **UI Component Library:** Vuetify.
- **Dependencies:** `axios` (API requests), `echarts` / `vue-chartjs` (Data visualization).
- **Build Output:** `web/dist/` (Served by the Go backend).

## Project Structure

```text
lemwood_mirror/
├── cmd/
│   └── mirror/       # Main Go application entry point
├── internal/         # Private application logic
│   ├── browser/      # Web scraping logic (repo resolution)
│   ├── config/       # Configuration loading
│   ├── db/           # Database initialization
│   ├── downloader/   # File downloading logic
│   ├── github/       # GitHub API client wrapper
│   ├── server/       # HTTP server and API routes
│   ├── stats/        # Statistics gathering and reporting
│   └── storage/      # File storage management
├── web/              # Vue 3 Frontend source
│   ├── src/          # Frontend source code
│   ├── dist/         # Compiled frontend assets (served by backend)
│   └── vite.config.js # Vite configuration
├── config.json       # Application configuration (example/template)
├── go.mod            # Go dependencies
└── README.md         # Project documentation
```

## Building and Running

### Prerequisites
- **Go:** Version 1.24 or later.
- **Node.js:** Required to build the frontend.

### 1. Build Frontend
The Go backend expects the frontend artifacts to be present in `web/dist`.

```bash
cd web
npm install
npm run build
cd ..
```

### 2. Build Backend

```bash
# Windows
go build -o mirror.exe ./cmd/mirror

# Linux/macOS
go build -o mirror ./cmd/mirror
```

### 3. Configuration
Ensure a `config.json` file is present in the working directory. Key fields include:
- `github_token`: GitHub Personal Access Token (crucial for API rate limits).
- `storage_path`: Directory to save downloaded files.
- `launchers`: List of software to mirror (name, source URL, selector).
- `server_port`: Port to listen on (default 8080).

### 4. Run

```bash
./mirror
```

## Development Conventions

- **Safe File Serving:** The server implementation (`internal/server/server.go`) includes explicit checks to prevent path traversal attacks (e.g., `containsDotDot`, path prefix validation). Maintain these checks when modifying file serving logic.
- **Concurrency:** The downloader and scanner use mutexes (`sync.Mutex`) to manage concurrent access to state and ensure atomic updates.
- **API Design:**
    - `/api/status`: Returns all version information.
    - `/api/latest`: Returns the latest version for each launcher.
    - `/api/stats`: Returns usage statistics.
    - `/download/...`: Serves the actual files.
- **Frontend-Backend Integration:** The Go server mounts the `web/dist` directory to serve static assets. API requests from the frontend are proxied or handled directly by the same origin.
