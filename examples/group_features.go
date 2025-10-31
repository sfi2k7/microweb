package main

import (
	"log"
	"net/http"

	"github.com/sfi2k7/microweb"
)

func main() {
	router := microweb.New()

	// ====================================
	// Basic Router-level Routes
	// ====================================
	router.Get("/", func(ctx *microweb.Context) {
		ctx.String("Welcome to the home page!")
	})

	// Any method - handles all HTTP methods
	router.Any("/ping", func(ctx *microweb.Context) {
		ctx.Json(map[string]string{
			"message": "pong",
			"method":  ctx.Method,
		})
	})

	// Match specific methods
	router.Match([]string{"GET", "POST"}, "/contact", func(ctx *microweb.Context) {
		if ctx.Method == "GET" {
			ctx.String("Contact form")
		} else {
			ctx.String("Form submitted!")
		}
	})

	// ====================================
	// API Group with Middleware
	// ====================================
	api := router.Group("/api")

	// Add logging middleware to entire API group
	api.Use(func(ctx *microweb.Context) bool {
		log.Printf("[API] %s %s", ctx.Method, ctx.R.URL.Path)
		return true
	})

	// Add authentication middleware to API group
	api.Use(func(ctx *microweb.Context) bool {
		token := ctx.Header("X-API-Key")
		if token == "" {
			ctx.Status(http.StatusUnauthorized)
			ctx.Json(map[string]string{"error": "Missing API key"})
			return false
		}
		// Store user info in context
		ctx.Set("authenticated", true)
		return true
	})

	api.Get("/status", func(ctx *microweb.Context) {
		ctx.Json(map[string]interface{}{
			"status":        "ok",
			"authenticated": ctx.Get("authenticated"),
		})
	})

	// ====================================
	// Nested Groups - API Versioning
	// ====================================
	v1 := api.Group("/v1")

	// V1 specific middleware
	v1.Use(func(ctx *microweb.Context) bool {
		ctx.Set("version", "v1")
		return true
	})

	v1.Get("/users", func(ctx *microweb.Context) {
		ctx.Json(map[string]interface{}{
			"version": ctx.Get("version"),
			"users":   []string{"Alice", "Bob", "Charlie"},
		})
	})

	v1.Post("/users", func(ctx *microweb.Context) {
		var user map[string]string
		if err := ctx.Parse(&user); err != nil {
			ctx.Status(http.StatusBadRequest)
			ctx.Json(map[string]string{"error": "Invalid JSON"})
			return
		}
		ctx.Status(http.StatusCreated)
		ctx.Json(map[string]interface{}{
			"version": ctx.Get("version"),
			"user":    user,
		})
	})

	// V1 products subgroup
	products := v1.Group("/products")
	products.Get("/", func(ctx *microweb.Context) {
		ctx.Json(map[string]interface{}{
			"version":  ctx.Get("version"),
			"products": []string{"Product A", "Product B"},
		})
	})

	products.Get("/{id}", func(ctx *microweb.Context) {
		id := ctx.Param("id")
		ctx.Json(map[string]interface{}{
			"version":    ctx.Get("version"),
			"product_id": id,
			"name":       "Product " + id,
		})
	})

	// ====================================
	// V2 API Group
	// ====================================
	v2 := api.Group("/v2")

	v2.Use(func(ctx *microweb.Context) bool {
		ctx.Set("version", "v2")
		return true
	})

	v2.Get("/users", func(ctx *microweb.Context) {
		ctx.Json(map[string]interface{}{
			"version": ctx.Get("version"),
			"users": []map[string]interface{}{
				{"id": 1, "name": "Alice"},
				{"id": 2, "name": "Bob"},
				{"id": 3, "name": "Charlie"},
			},
		})
	})

	// Any method on V2 products
	v2.Any("/products", func(ctx *microweb.Context) {
		ctx.Json(map[string]interface{}{
			"version": ctx.Get("version"),
			"method":  ctx.Method,
			"message": "V2 products endpoint",
		})
	})

	// ====================================
	// Admin Group with Additional Middleware
	// ====================================
	admin := router.Group("/admin")

	// Admin authentication (runs first)
	admin.Use(func(ctx *microweb.Context) bool {
		token := ctx.Header("Admin-Token")
		if token != "admin123" {
			ctx.Status(http.StatusForbidden)
			ctx.Json(map[string]string{"error": "Admin access denied"})
			return false
		}
		return true
	})

	admin.Get("/dashboard", func(ctx *microweb.Context) {
		ctx.String("Admin Dashboard")
	})

	admin.Match([]string{"GET", "POST", "DELETE"}, "/users", func(ctx *microweb.Context) {
		ctx.Json(map[string]string{
			"action": "Admin user management",
			"method": ctx.Method,
		})
	})

	// ====================================
	// Static Files in Group
	// ====================================
	assets := router.Group("/assets")
	assets.Static("./public/assets") // Serves files from ./public/assets at /assets/*

	// ====================================
	// UseOnly - Middleware for specific route only
	// ====================================
	special := router.Group("/special")

	// This middleware only applies to the specific handler, not the whole group
	rateLimitMiddleware := func(ctx *microweb.Context) bool {
		// Simple rate limiting check
		log.Println("Rate limit check for:", ctx.R.URL.Path)
		return true
	}

	special.Get("/limited", special.UseOnly(
		func(ctx *microweb.Context) {
			ctx.String("This endpoint has specific rate limiting")
		},
		rateLimitMiddleware,
	))

	special.Get("/unlimited", func(ctx *microweb.Context) {
		ctx.String("This endpoint doesn't have rate limiting")
	})

	// ====================================
	// Route Debugging - List all routes
	// ====================================
	router.Get("/debug/routes", func(ctx *microweb.Context) {
		allRoutes := router.Routes()
		ctx.Json(map[string]interface{}{
			"total":  len(allRoutes),
			"routes": allRoutes,
		})
	})

	// List routes for specific groups
	router.Get("/debug/api-routes", func(ctx *microweb.Context) {
		apiRoutes := api.AllRoutes()
		ctx.Json(map[string]interface{}{
			"group":  api.Prefix(),
			"total":  len(apiRoutes),
			"routes": apiRoutes,
		})
	})

	router.Get("/debug/v1-routes", func(ctx *microweb.Context) {
		v1Routes := v1.Routes() // Only direct routes, not children
		ctx.Json(map[string]interface{}{
			"group":  v1.Prefix(),
			"total":  len(v1Routes),
			"routes": v1Routes,
		})
	})

	// ====================================
	// Example: Public vs Protected Resources
	// ====================================
	blog := router.Group("/blog")

	// Public routes (no middleware)
	blog.Get("/posts", func(ctx *microweb.Context) {
		ctx.Json([]map[string]string{
			{"id": "1", "title": "First Post"},
			{"id": "2", "title": "Second Post"},
		})
	})

	blog.Get("/posts/{id}", func(ctx *microweb.Context) {
		id := ctx.Param("id")
		ctx.Json(map[string]string{
			"id":      id,
			"title":   "Post " + id,
			"content": "Content here...",
		})
	})

	// Protected blog management
	blogAdmin := blog.Group("/admin")
	blogAdmin.Use(func(ctx *microweb.Context) bool {
		if ctx.Header("Blog-Admin") != "secret" {
			ctx.Status(http.StatusUnauthorized)
			ctx.Json(map[string]string{"error": "Not authorized"})
			return false
		}
		return true
	})

	blogAdmin.Post("/posts", func(ctx *microweb.Context) {
		var post map[string]string
		ctx.Parse(&post)
		ctx.Status(http.StatusCreated)
		ctx.Json(map[string]string{
			"message": "Post created",
			"title":   post["title"],
		})
	})

	blogAdmin.Delete("/posts/{id}", func(ctx *microweb.Context) {
		id := ctx.Param("id")
		ctx.Json(map[string]string{
			"message": "Post deleted",
			"id":      id,
		})
	})

	// ====================================
	// Print all registered routes on startup
	// ====================================
	log.Println("Registered routes:")
	for _, route := range router.Routes() {
		log.Printf("  %s", route)
	}

	// ====================================
	// Start Server
	// ====================================
	log.Println("\nServer starting on :8080")
	log.Println("\nTry these endpoints:")
	log.Println("  GET  http://localhost:8080/")
	log.Println("  GET  http://localhost:8080/ping")
	log.Println("  GET  http://localhost:8080/api/v1/users (needs X-API-Key header)")
	log.Println("  GET  http://localhost:8080/api/v2/users (needs X-API-Key header)")
	log.Println("  GET  http://localhost:8080/admin/dashboard (needs Admin-Token: admin123)")
	log.Println("  GET  http://localhost:8080/blog/posts")
	log.Println("  GET  http://localhost:8080/debug/routes")
	log.Println("  GET  http://localhost:8080/debug/api-routes")
	log.Println()

	if err := router.Listen(8080); err != nil {
		log.Fatal(err)
	}
}
