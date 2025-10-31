package main

import (
	"log"
	"time"

	"github.com/sfi2k7/microweb"
)

func main() {
	router := microweb.New()

	// ====================================
	// HTTP Routes
	// ====================================
	router.Get("/", func(ctx *microweb.Context) {
		ctx.String("WebSocket Example Server")
	})

	// Send message to specific client from HTTP endpoint
	router.Post("/notify/{clientId}", func(ctx *microweb.Context) {
		clientId := ctx.Param("clientId")
		message := ctx.FormValue("message")

		microweb.Hub.Send(clientId, map[string]string{
			"type":    "notification",
			"message": message,
		})

		ctx.Json(map[string]interface{}{
			"sent":     true,
			"clientId": clientId,
		})
	})

	// Broadcast to all clients from HTTP endpoint
	router.Post("/broadcast", func(ctx *microweb.Context) {
		message := ctx.FormValue("message")

		microweb.Hub.Broadcast(map[string]string{
			"type":    "broadcast",
			"message": message,
		})

		ctx.Json(map[string]interface{}{
			"sent":  true,
			"count": microweb.Hub.Count(),
		})
	})

	// Get connected clients count
	router.Get("/clients/count", func(ctx *microweb.Context) {
		ctx.Json(map[string]interface{}{
			"count": microweb.Hub.Count(),
		})
	})

	// ====================================
	// WebSocket Route
	// ====================================
	router.Ws("/ws", func(ctx *microweb.ClientContext) *microweb.WsData {
		// Connection lifecycle hooks
		ctx.On("open", func(c *microweb.ClientContext) {
			log.Printf("Client connected: %s", c.Id)

			// Send welcome message
			c.Send(map[string]string{
				"type":     "welcome",
				"clientId": c.Id,
				"message":  "Connected successfully",
			})
		})

		ctx.On("close", func(c *microweb.ClientContext) {
			log.Printf("Client disconnected: %s", c.Id)
		})

		ctx.On("error", func(c *microweb.ClientContext) {
			log.Printf("Client error: %s - %v", c.Id, c.Data.Get("error"))
		})

		// Handle incoming messages
		cmd := ctx.Data.String("cmd")

		switch cmd {
		case "ping":
			// Simple ping-pong
			return microweb.NewWsDataFromMap(map[string]interface{}{
				"type":     "pong",
				"time":     time.Now().Unix(),
				"clientId": ctx.Id,
			})

		case "echo":
			// Echo back the message
			message := ctx.Data.String("message")
			return microweb.NewWsDataFromMap(map[string]interface{}{
				"type":    "echo",
				"message": message,
			})

		case "broadcast":
			// Broadcast to all other clients
			message := ctx.Data.String("message")
			microweb.Hub.Broadcast(map[string]interface{}{
				"type":    "broadcast",
				"from":    ctx.Id,
				"message": message,
				"time":    time.Now().Unix(),
			})
			return nil // No reply to sender

		case "send":
			// Send to specific client
			targetId := ctx.Data.String("targetId")
			message := ctx.Data.String("message")

			microweb.Hub.Send(targetId, map[string]interface{}{
				"type":    "private",
				"from":    ctx.Id,
				"message": message,
			})

			return microweb.NewWsDataFromMap(map[string]interface{}{
				"type":   "sent",
				"target": targetId,
			})

		case "stats":
			// Get server stats
			return microweb.NewWsDataFromMap(map[string]interface{}{
				"type":           "stats",
				"connectedCount": microweb.Hub.Count(),
				"yourId":         ctx.Id,
			})

		case "disconnect":
			// Close this connection
			ctx.Close()
			return nil

		default:
			// Unknown command
			return microweb.NewWsDataFromMap(map[string]interface{}{
				"type":  "error",
				"error": "Unknown command: " + cmd,
			})
		}
	})

	// ====================================
	// Background Task Example
	// ====================================
	go func() {
		ticker := time.NewTicker(30 * time.Second)
		defer ticker.Stop()

		for range ticker.C {
			count := microweb.Hub.Count()
			if count > 0 {
				log.Printf("Broadcasting server time to %d clients", count)
				microweb.Hub.Broadcast(map[string]interface{}{
					"type":    "serverTime",
					"time":    time.Now().Unix(),
					"message": "Server heartbeat",
				})
			}
		}
	}()

	// ====================================
	// Start Server
	// ====================================
	log.Println("WebSocket Server starting on :8080")
	log.Println("\nWebSocket endpoint: ws://localhost:8080/ws")
	log.Println("\nHTTP endpoints:")
	log.Println("  GET  http://localhost:8080/")
	log.Println("  POST http://localhost:8080/notify/{clientId}?message=hello")
	log.Println("  POST http://localhost:8080/broadcast?message=hello")
	log.Println("  GET  http://localhost:8080/clients/count")
	log.Println("\nWebSocket commands (send as JSON):")
	log.Println(`  {"cmd": "ping"}`)
	log.Println(`  {"cmd": "echo", "message": "hello"}`)
	log.Println(`  {"cmd": "broadcast", "message": "hello all"}`)
	log.Println(`  {"cmd": "send", "targetId": "clientId", "message": "private msg"}`)
	log.Println(`  {"cmd": "stats"}`)
	log.Println(`  {"cmd": "disconnect"}`)
	log.Println()

	if err := router.Listen(8080); err != nil {
		log.Fatal(err)
	}
}
