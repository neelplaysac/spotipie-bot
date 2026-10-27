# Spotipie bot

A Telegram bot that shares Spotify and Last.fm listening cards. Runtime settings
are listed in [.env.example](.env.example). Supply them as environment variables
or in a local `.env` file.

## Structure

```text
cmd/spotipie/          Executable entry point and flags
internal/app/         Dependency wiring, startup, and shutdown
internal/config/      Environment configuration
internal/telegram/    Commands, callbacks, inline queries, and conversations
internal/service/     Account and playback workflows, models, and interfaces
internal/storage/mongo/ MongoDB documents and repository implementation
internal/spotify/     Spotify API client
internal/lastfm/      Last.fm API client
internal/card/        Listening-card rendering
internal/observability/ Logging, credential redaction, and Sentry
fonts/                Runtime font assets
```

Telegram handlers call services; services depend on small interfaces implemented
by MongoDB, the API clients, and the renderer. `internal/app` constructs these
dependencies and passes them into the handlers. Services do not import Telegram,
MongoDB, image-rendering libraries, or Sentry.

## Build and run

Run commands from the repository root so `fonts/` resolves correctly:

```bash
go run ./cmd/spotipie
go build -o bin/spotipie-go ./cmd/spotipie
go run ./cmd/spotipie -sentry-test
```

The Docker binary path remains `/app/spotipie-go`, and the existing environment
variables, MongoDB collections, and Telegram commands are unchanged.

## Checks

```bash
go test ./...
go test -race ./...
go vet ./...
```
