# petroapp

Station transfer event ingestion and reconciliation service.

## Tech stack

- **Go 1.25**
- **Gin** — HTTP framework
- **go-playground/validator** — request validation
- **logrus** — structured logging
- In-memory store (thread-safe, no external dependencies)

## Configuration

| Variable | Default | Description |
|---|---|---|
| `PORT` | `8080` | Port the HTTP server listens on |

## Run locally

```bash
make run
# or
go run .

# custom port
make run PORT=9090
# or
PORT=9090 go run .
```

Server starts on `http://localhost:8080` (or whichever port you set).

## Run tests

```bash
make test
# or
go test ./...
```

## Run with Docker

```bash
# build and start on default port 8080
docker compose up --build

# build and start on a custom port
PORT=9090 docker compose up --build

# run tests inside a container
docker compose run --rm app sh -c "go test ./..."
```

## API

### POST /transfers

Ingest a batch of transfer events. Valid events are inserted; invalid ones are skipped and counted.

```bash
curl -X POST http://localhost:8080/transfers \
  -H "Content-Type: application/json" \
  -d '{
    "events": [
      {
        "event_id": "evt-001",
        "station_id": "S1",
        "amount": 150.75,
        "status": "approved",
        "created_at": "2026-02-19T10:00:00Z"
      },
      {
        "event_id": "evt-002",
        "station_id": "S1",
        "amount": 200.00,
        "status": "approved",
        "created_at": "2026-02-19T11:00:00Z"
      }
    ]
  }'
```

Response:

```json
{
  "inserted": 2,
  "duplicates": 0,
  "invalid": 0
}
```

### GET /stations/:station_id/summary

```bash
curl http://localhost:8080/stations/S1/summary
```

Response:

```json
{
  "station_id": "S1",
  "total_approved_amount": 350.75,
  "events_count": 2
}
```

A Postman collection (`petroapp.postman_collection.json`) is included. Import it and set `base_url` to `http://localhost:8080`.

---

## Design notes

### Idempotency

Each event carries a globally unique `event_id`. The in-memory store maintains a `UniqueIndex map[string]struct{}` that acts as a unique constraint. `InsertEvent` checks the index before writing; if the ID is already present it returns `(false, nil)` and the caller counts it as a duplicate without modifying any data.

### Concurrency

`InsertEvent` holds a full **write lock** (`sync.Mutex`) for the entire check-then-insert sequence. This eliminates the TOCTOU race: two goroutines arriving with the same `event_id` at the same time will serialize at the lock — the second one will find the ID already in the index and skip the insert.

Read operations (`GetStationEventsByStationId`) use a **read lock** (`sync.RWMutex`), so summary queries don't block each other.

### Batch validation strategy — partial accept

Invalid events (missing required fields, negative amount, unparseable `created_at`) are **skipped** rather than failing the whole batch. The rationale: a batch from an external system may mix good and bad records; rejecting everything forces the caller to resend all the valid events too, which increases duplication risk. Skipped events are logged as warnings and counted in the `invalid` field of the response so the caller knows exactly what happened.

### events_count

`events_count` in the summary reflects **all stored events for that station regardless of status**. Only `total_approved_amount` filters by `status == "approved"`. This gives a full picture of ingestion volume alongside the financial reconciliation figure.

### Tradeoffs

| Decision | Choice | Alternative |
|---|---|---|
| Storage | In-memory (custom) | HashiCorp's `go-memdb` would provide a richer in-memory store with indexing and transactions out of the box, but a plain map + mutex is sufficient here and avoids the extra dependency. For persistence, SQLite/Postgres would survive restarts and scale horizontally. |
| Uniqueness | Hash map + mutex | DB unique constraint — automatic, but adds infra dependency |
| Batch errors | Partial accept | Fail-fast — simpler caller contract, but wastes valid data on one bad record |
