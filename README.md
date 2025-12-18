```md
# WebSocket → PostgreSQL Benchmark (Go)

This project explores and benchmarks different strategies for ingesting
high-volume WebSocket messages into PostgreSQL using Go.

It was built to answer a simple but important question:

> How much do buffering and batching really matter when writing millions of
> real-time messages to a database?

The answer, as you’ll see, is **a lot**.

---

## 🚀 Overview

The system consists of two main components:

- **Server**: A WebSocket server that receives messages and persists them into PostgreSQL
- **Client**: A load generator that simulates many concurrent WebSocket clients

Three ingestion strategies are implemented and benchmarked:

1. **Naive** – synchronous INSERT per message
2. **Buffered** – asynchronous worker pool with single inserts
3. **Bulk** – batched INSERTs using multi-value SQL statements

---

## 🧪 Benchmark Setup

- **Clients**: 10 concurrent WebSocket clients
- **Messages per client**: 100,000
- **Total messages**: 1,000,000
- **Database**: PostgreSQL (local)
- **Machine**:
  - CPU: Intel i7-13650HX
  - RAM: 16 GB

---

## 📊 Results

| Mode      | Description                              | Time     |
|-----------|------------------------------------------|----------|
| Naive     | Sync INSERT per message                  | ~1m30s   |
| Buffered  | Async workers, single INSERT             | ~1m10s   |
| Bulk      | Batched INSERT (5,000 messages per batch)| ~7s      |

The bulk insert approach reduces database round trips dramatically and
achieves more than **10× performance improvement** compared to the naive
implementation.

---

## 🧠 Key Learnings

- Database round trips are often the dominant bottleneck in ingestion systems
- Concurrency alone helps, but does not eliminate per-query overhead
- Batching writes is one of the most effective optimizations for high-throughput systems
- Simple architectural changes can outperform hardware scaling

---

## 📁 Project Structure

```

.
├── server/        # WebSocket ingestion server
│   └── server.go
│
├── client/        # WebSocket load generator
│   └── client.go
│
└── README.md

````

---

## 🖥️ Running the Project

### 1. Start PostgreSQL

Make sure PostgreSQL is running locally and update the DSN in
`server/server.go` if needed.

---

### 2. Run the Server

```bash
cd server
go run server.go -mode=bulk
````

Available modes:

* `naive`
* `buffered`
* `bulk`

---

### 3. Run the Client

In a separate terminal:

```bash
cd client
go run client.go
```

The client will spawn multiple WebSocket connections and send messages
concurrently.

---

## 📝 Notes

* The server automatically recreates the `messages` table on startup
* Batch size and worker count can be tuned in the server configuration
* The project is intentionally simple to make performance bottlenecks visible

---

## 🧩 Use Cases

* Learning high-throughput ingestion patterns
* Benchmarking database write strategies
* Understanding WebSocket backpressure and buffering
* Reference implementation for Go concurrency patterns

```
