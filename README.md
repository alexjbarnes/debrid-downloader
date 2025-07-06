# Debrid Downloader

A modern web application for downloading files through AllDebrid services, built with Go and featuring a responsive UI with real-time updates.

![Go Version](https://img.shields.io/badge/Go-1.24.4-00ADD8?style=flat&logo=go)
![HTMX](https://img.shields.io/badge/HTMX-2.0.4-3D72D7?style=flat)
![License](https://img.shields.io/badge/license-MIT-blue.svg)

## Features

### 🚀 Core Functionality
- **AllDebrid Integration** - Seamlessly unrestrict premium file hosts
- **Smart Downloads** - Automatic retry, pause/resume, and progress tracking
- **Archive Support** - Automatic extraction of RAR archives with file tracking
- **Batch Operations** - Download multiple files simultaneously

### 🎯 Intelligent Features
- **Directory Learning** - ML-like system that suggests directories based on your usage patterns
- **Fuzzy Search** - Quickly find downloads in your history
- **Auto-Cleanup** - Removes old downloads after 60 days
- **Real-time Updates** - Live progress without page refreshes using HTMX

### 🎨 Modern UI
- **Responsive Design** - Works on desktop and mobile devices
- **Dark/Light Mode** - Automatic theme detection with manual override
- **Drag & Drop** - Easy file URL input
- **Keyboard Shortcuts** - Efficient navigation

## Quick Start

### Using Docker

```bash
docker run -d \
  -p 8080:8080 \
  -e ALLDEBRID_API_KEY=your_api_key \
  -v ./downloads:/downloads \
  -v ./data:/data \
  ghcr.io/yourusername/debrid-downloader:latest
```

### Using Docker Compose

```yaml
version: '3.8'

services:
  debrid-downloader:
    image: ghcr.io/yourusername/debrid-downloader:latest
    ports:
      - "8080:8080"
    environment:
      - ALLDEBRID_API_KEY=your_api_key
      - DATABASE_PATH=/data/debrid.db
      - BASE_DOWNLOADS_PATH=/downloads
    volumes:
      - ./downloads:/downloads
      - ./data:/data
    restart: unless-stopped
```

### Building from Source

```bash
# Clone the repository
git clone https://github.com/yourusername/debrid-downloader.git
cd debrid-downloader

# Install dependencies
just install-tools

# Build the application
just build

# Run with environment variables
ALLDEBRID_API_KEY=your_api_key ./bin/debrid-downloader
```

## Configuration

Create a `.env` file or set environment variables:

```bash
# Required
ALLDEBRID_API_KEY=your_api_key_here

# Optional
SERVER_PORT=8080                    # Web server port
DATABASE_PATH=debrid.db            # SQLite database location
BASE_DOWNLOADS_PATH=/downloads     # Base directory for downloads
LOG_LEVEL=info                     # Logging level (debug|info|warn|error)
```

## Development

### Prerequisites
- Go 1.24.4 or higher
- Just (command runner)
- Node.js (for Tailwind CSS if customizing styles)

### Setup Development Environment

```bash
# Install development tools
just install-tools

# Run development server with auto-reload
just run

# Run tests
just test

# Run all quality checks
just check
```

### Available Commands

```bash
just build              # Build with templ generation
just run                # Run development server  
just test               # Run tests with race detection and coverage
just coverage           # Detailed coverage analysis
just check              # Full quality checks (format, lint, test)
just templ-generate     # Generate templ templates to Go code
just fmt                # Format code with gofumpt
just lint               # Run golangci-lint and staticcheck
just mocks              # Generate mocks
```

## Architecture

### Directory Structure
```
debrid-downloader/
├── cmd/debrid-downloader/    # Main application entry
├── internal/                 # Core business logic
│   ├── alldebrid/           # AllDebrid API client
│   ├── config/              # Configuration management
│   ├── database/            # SQLite operations
│   ├── downloader/          # Download worker
│   ├── extractor/           # Archive extraction
│   ├── folder/              # Secure folder browsing
│   └── web/                 # HTTP server & handlers
├── pkg/                     # Shared packages
│   ├── fuzzy/              # Fuzzy matching
│   └── models/             # Data models
└── templates/              # Templ templates
```

### Database Schema
- **downloads** - Tracks download lifecycle and metadata
- **directory_mappings** - Learns directory preferences for intelligent suggestions

## API Endpoints

- `GET /` - Main download interface
- `GET /history` - Download history with search
- `GET /settings` - Application settings
- `POST /download` - Submit new download
- `GET /api/folders` - Browse folders (AJAX)
- `GET /api/downloads` - Get downloads (AJAX)
- `POST /api/downloads/{id}/pause` - Pause download
- `POST /api/downloads/{id}/resume` - Resume download
- `POST /api/downloads/{id}/retry` - Retry failed download

## Security Features

- Path traversal protection in folder browser
- API key validation on startup
- Input sanitization for all user inputs
- Non-root container execution
- Secure session handling

## Contributing

1. Fork the repository
2. Create your feature branch (`git checkout -b feature/amazing-feature`)
3. Commit your changes (`git commit -m 'Add amazing feature'`)
4. Push to the branch (`git push origin feature/amazing-feature`)
5. Open a Pull Request

Please ensure:
- All tests pass (`just test`)
- Code is formatted (`just fmt`)
- Linting passes (`just lint`)

## License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.

## Acknowledgments

- Built with [HTMX](https://htmx.org/) for progressive enhancement
- Styled with [Tailwind CSS](https://tailwindcss.com/)
- Templates powered by [Templ](https://templ.guide/)
- Container images from [Chainguard](https://chainguard.dev/)