# multi-window-media-sequencer

Full-stack Multi-Window Media Sequencer with Synchronized Playback.

## Architecture Overview
- **Backend**: Golang (Go 1.22+) with clean architecture, REST API, MongoDB persistent storage, and WebSocket support.
- **Frontend**: React (Vite) with multi-window display grid, gapless dual-buffer media player, and admin control panels.
- **Database**: MongoDB 7.0 (authoritative persistent storage).

## Project Structure
- `backend/`: Go server application, MongoDB repository layer, models, handlers, and test suites.
- `docker-compose.yml`: Multi-container configuration for local MongoDB and Backend services.
- `Backend_Intern_Assignment.pdf`: Assignment specification document.

## Quick Start
See [backend/README.md](backend/README.md) for backend setup instructions, API documentation, and test execution details.
