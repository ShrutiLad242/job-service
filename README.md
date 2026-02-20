# Job Handler

A small concurrent background job processing system in Go.

## Overview

This project demonstrates a simple worker-pool based job processing system with a HTTP API for submitting, listing, retrieving, and cancelling jobs.

Key components:

- `main.go` — bootstraps the `JobStore`, `WorkerPool`, and HTTP server (Gin).
- `handler/` — HTTP handlers for creating, listing, retrieving, and cancelling jobs.
- `model/` — `Job` model and status constants.
- `store/` — in-memory `JobStore` (map + RWMutex).
- `worker/` — `WorkerPool` implementation that processes jobs concurrently.

This is intentionally minimal and in-memory; see "Roadmap / Improvements" for production recommendations.

---

## How it works (high level)

1. API client POSTs to `/jobs` with a `payload` string.
2. The `CreateJob` handler creates a `model.Job` with a cancellable `context.Context`, stores it in the `JobStore`, and enqueues it on the `WorkerPool`.
3. The `WorkerPool` has N worker goroutines reading from a buffered channel. For each job:
   - If the job's context is already cancelled, the job is marked `cancelled`.
   - Otherwise the job is marked `processing` and `processJob()` is executed.
   - On success the job is marked `completed` and `Result` is set. On error it is marked `failed` and `Error` is set.
4. `CancelJob` calls the job's stored `CancelFunc()` and updates its status to `cancelled`.

Notes: `store.JobStore` uses a `sync.RWMutex` to protect the internal map. `job` fields are updated directly by both handlers and workers.

---

## Running locally

Prerequisites:
- Go 1.25+ installed (module-aware)

Build and run server:

```bash
cd /path/to/job-handler
go run main.go
# Server listens on :8080
```

API endpoints (examples):

- Create job:

```bash
curl -X POST -H "Content-Type: application/json" -d '{"payload":"hello"}' http://localhost:8080/jobs
```

- Get job:

```bash
curl http://localhost:8080/jobs/<job-id>
```

- List jobs:

```bash
curl http://localhost:8080/jobs
```

- Cancel job:

```bash
curl -X POST http://localhost:8080/jobs/<job-id>/cancel
```

---

## Running tests

Run all tests in the repository:

```bash
cd /path/to/job-handler
go test ./... -v
```

### Test speed

Unit tests in `worker` use a package-level variable `processJobDelay` to simulate processing time. Tests set this to a small value in `TestMain` so the suite runs quickly. If you change processing timing for local debugging, be aware tests rely on a short delay.

---

## Project layout

- `main.go` — app entrypoint
- `handler/` — HTTP handler layer
- `model/` — job model and constants
- `store/` — in-memory job store (map + RWMutex)
- `worker/` — worker pool and job processing logic
- `ANALYSIS.md` — analysis and roadmap for improvements

---

## Development tips

- To speed tests, tests in `worker` set `processJobDelay` to 10ms in `TestMain`.
- To experiment with different worker counts or queue sizes, change `NewWorkerPool` construction in `main.go`.

