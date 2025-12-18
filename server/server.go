// ============================================
// WebSocket → PostgreSQL Benchmark Server
// Modes:
//   - naive    : sync INSERT per message
//   - buffered : async workers, single INSERT
//   - bulk     : batched INSERT
//
// Usage:
//   go run main.go -mode=naive
//   go run main.go -mode=buffered
//   go run main.go -mode=bulk
// ============================================

package main

import (
	"database/sql"
	"flag"
	"fmt"
	"log"
	"runtime"
	"strings"
	"sync/atomic"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/websocket/v2"
	_ "github.com/lib/pq"
)

const (
	channelBufferSize = 500_000
	workerCount       = 4
	batchSize         = 5_000
	flushInterval     = 300 * time.Millisecond
)

var (
	clientCount       = 10
	messagesPerClient = 100_000
	totalMessages     = clientCount * messagesPerClient
	insertedCount     int64
	startTime         time.Time
)

func main() {
	mode := flag.String("mode", "naive", "naive | buffered | bulk")
	flag.Parse()

	log.Printf("Starting server in %s mode", *mode)

	app := fiber.New()

	switch *mode {
	case "naive":
		runNaive(app)
	case "buffered":
		runBuffered(app)
	case "bulk":
		runBulk(app)
	default:
		log.Fatalf("unknown mode: %s", *mode)
	}

	log.Println("WebSocket server started on :8080")
	log.Fatal(app.Listen(":8080"))
}

// ============================================
// Mode 1 – Naive (sync INSERT)
// ============================================
func runNaive(app *fiber.App) {
	db := mustSQL()

	startTime = time.Now()

	app.Use("/ws", websocket.New(func(c *websocket.Conn) {
		for {
			_, msg, err := c.ReadMessage()
			if err != nil {
				return
			}

			_, err = db.Exec("INSERT INTO messages (payload) VALUES ($1)", string(msg))
			if err != nil {
				log.Println("db error:", err)
			}

			atomic.AddInt64(&insertedCount, 1)
			if atomic.LoadInt64(&insertedCount) == int64(totalMessages) {
				log.Printf("Naive mode done! Total time: %v", time.Since(startTime))
			}
		}
	}))
}

// ============================================
// Mode 2 – Buffered workers (async INSERT)
// ============================================
func runBuffered(app *fiber.App) {
	db := mustSQL()
	ch := make(chan string, channelBufferSize)

	startTime = time.Now()

	for i := 0; i < workerCount; i++ {
		go func(id int) {
			for msg := range ch {
				_, err := db.Exec("INSERT INTO messages (payload) VALUES ($1)", msg)
				if err != nil {
					log.Printf("worker %d error: %v", id, err)
				}

				atomic.AddInt64(&insertedCount, 1)
				if atomic.LoadInt64(&insertedCount) == int64(totalMessages) {
					log.Printf("Buffered mode done! Total time: %v", time.Since(startTime))
				}
			}
		}(i)
	}

	log.Printf("Buffered mode: workers=%d CPU=%d", workerCount, runtime.NumCPU())

	app.Use("/ws", websocket.New(func(c *websocket.Conn) {
		for {
			_, msg, err := c.ReadMessage()
			if err != nil {
				return
			}
			ch <- string(msg)
		}
	}))
}

// ============================================
// Mode 3 – Bulk INSERT (batched)
// ============================================
func runBulk(app *fiber.App) {
	db := mustSQL()
	ch := make(chan string, channelBufferSize)

	startTime = time.Now()

	for i := range workerCount {
		go bulkInsertWorker(i, db, ch)
	}

	log.Printf("Bulk INSERT mode: workers=%d batch=%d", workerCount, batchSize)

	app.Use("/ws", websocket.New(func(c *websocket.Conn) {
		for {
			_, msg, err := c.ReadMessage()
			if err != nil {
				return
			}
			ch <- string(msg)
		}
	}))
}

// ============================================
// Helpers
// ============================================
func mustSQL() *sql.DB {
	dsn := "postgres://postgres:postgres@localhost:5432/ws_demo?sslmode=disable"
	db, err := sql.Open("postgres", dsn)
	if err != nil {
		log.Fatal(err)
	}

	// Drop the table if it exists, along with its dependencies
	if _, err := db.Exec(`DROP TABLE IF EXISTS messages CASCADE`); err != nil {
		log.Fatalf("failed to drop table: %v", err)
	}

	// Create the table with proper schema
	if _, err := db.Exec(`
    CREATE TABLE IF NOT EXISTS messages (
        id SERIAL PRIMARY KEY,
        payload TEXT NOT NULL,
        created_at TIMESTAMP DEFAULT NOW()
    )
	`); err != nil {
		log.Fatalf("failed to create table: %v", err)
	}

	return db
}

func bulkInsertWorker(id int, db *sql.DB, ch <-chan string) {
	batch := make([]string, 0, batchSize)
	ticker := time.NewTicker(flushInterval)
	defer ticker.Stop()

	flush := func() {
		if len(batch) == 0 {
			return
		}

		query, args := buildBulkInsert(batch)
		if _, err := db.Exec(query, args...); err != nil {
			log.Printf("worker %d bulk insert error: %v", id, err)
		}

		atomic.AddInt64(&insertedCount, int64(len(batch)))
		if atomic.LoadInt64(&insertedCount) == int64(totalMessages) {
			log.Printf("Bulk mode done! Total time: %v", time.Since(startTime))
		}

		batch = batch[:0]
	}

	for {
		select {
		case msg := <-ch:
			batch = append(batch, msg)
			if len(batch) >= batchSize {
				flush()
			}

		case <-ticker.C:
			flush()
		}
	}
}

func buildBulkInsert(batch []string) (string, []any) {
	values := make([]string, 0, len(batch))
	args := make([]any, 0, len(batch))

	for i, msg := range batch {
		values = append(values, fmt.Sprintf("($%d)", i+1))
		args = append(args, msg)
	}

	query := "INSERT INTO messages (payload) VALUES " + strings.Join(values, ",")
	return query, args
}
