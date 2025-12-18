// ============================================
// Multi-Client WebSocket Load Generator
// ============================================
//
// This client:
// - Spawns multiple WebSocket clients
// - Each client sends messages concurrently
// - Used to stress-test the server ingestion pipeline

package main

import (
	"fmt"
	"log"
	"sync"

	"github.com/gorilla/websocket"
)

const (
	clientCount       = 10
	messagesPerClient = 100_000
)

func main() {
	var wg sync.WaitGroup

	wg.Add(clientCount)

	for i := range clientCount {
		go runClient(i, &wg)
	}

	wg.Wait()

	total := clientCount * messagesPerClient

	fmt.Printf("Sent %d messages using %d clients \n",
		total,
		clientCount,
	)
}

func runClient(id int, wg *sync.WaitGroup) {
	defer wg.Done()

	url := "ws://localhost:8080/ws"
	conn, _, err := websocket.DefaultDialer.Dial(url, nil)
	if err != nil {
		log.Printf("client %d dial error: %v\n", id, err)
		return
	}
	defer conn.Close()

	for i := range messagesPerClient {
		msg := fmt.Sprintf("client-%d-message-%d", id, i)
		if err := conn.WriteMessage(websocket.TextMessage, []byte(msg)); err != nil {
			log.Printf("client %d write error: %v\n", id, err)
			return
		}
	}
}
