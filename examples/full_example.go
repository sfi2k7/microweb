package main

import (
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/sfi2k7/microweb"
)

func main() {
	router := microweb.New()

	// ====================================
	// CORS Middleware
	// ====================================
	router.Use(microweb.CORS("*", "GET,POST,PUT,DELETE,OPTIONS", "Content-Type,Authorization"))

	// ====================================
	// Custom Panic Handler
	// ====================================
	router.SetPanicHandler(func(ctx *microweb.Context, err any) {
		log.Printf("Panic recovered: %v", err)
		ctx.Status(http.StatusInternalServerError)
		ctx.Json(map[string]string{
			"error": "Internal server error",
			"msg":   fmt.Sprintf("%v", err),
		})
	})

	// ====================================
	// Custom 404 Handler
	// ====================================
	router.SetNotFoundHandler(func(ctx *microweb.Context) {
		ctx.Status(http.StatusNotFound)
		ctx.Json(map[string]string{
			"error": "Route not found",
			"path":  ctx.R.URL.Path,
		})
	})

	// ====================================
	// Custom 405 Method Not Allowed Handler
	// ====================================
	router.SetMethodNotAllowedHandler(func(ctx *microweb.Context) {
		ctx.Status(http.StatusMethodNotAllowed)
		ctx.Json(map[string]string{
			"error":  "Method not allowed",
			"method": ctx.Method,
		})
	})

	// ====================================
	// Static File Serving
	// ====================================
	// Serve static files from /static prefix
	router.StaticWithPrefix("/static", "./public")
	// Serve root-level static files
	router.Static("./assets")

	// ====================================
	// Basic Routes
	// ====================================
	router.Get("/", func(ctx *microweb.Context) {
		ctx.String("Welcome to Microweb!")
	})

	router.Get("/hello/{name}", func(ctx *microweb.Context) {
		name := ctx.Param("name")
		ctx.Json(map[string]string{
			"message": "Hello, " + name,
		})
	})

	// ====================================
	// Cookie Examples
	// ====================================
	router.Get("/set-cookie", func(ctx *microweb.Context) {
		ctx.SetCookie(&http.Cookie{
			Name:     "user_session",
			Value:    "abc123xyz",
			Path:     "/",
			MaxAge:   3600,
			HttpOnly: true,
			Secure:   false,
		})
		ctx.String("Cookie set!")
	})

	router.Get("/get-cookie", func(ctx *microweb.Context) {
		cookie, err := ctx.Cookie("user_session")
		if err != nil {
			ctx.Status(http.StatusNotFound)
			ctx.Json(map[string]string{"error": "Cookie not found"})
			return
		}
		ctx.Json(map[string]string{
			"cookie_name":  cookie.Name,
			"cookie_value": cookie.Value,
		})
	})

	// ====================================
	// Redirect Example
	// ====================================
	router.Get("/redirect", func(ctx *microweb.Context) {
		ctx.Redirect("/", http.StatusFound)
	})

	// ====================================
	// File Upload Example
	// ====================================
	router.Post("/upload", func(ctx *microweb.Context) {
		file, header, err := ctx.FormFile("file")
		if err != nil {
			ctx.Status(http.StatusBadRequest)
			ctx.Json(map[string]string{"error": "Failed to get file"})
			return
		}

		// Save the file
		dst := "./uploads/" + header.Filename
		if err := ctx.SaveUploadedFile(file, header, dst); err != nil {
			ctx.Status(http.StatusInternalServerError)
			ctx.Json(map[string]string{"error": "Failed to save file"})
			return
		}

		ctx.Json(map[string]interface{}{
			"message":  "File uploaded successfully",
			"filename": header.Filename,
			"size":     header.Size,
			"path":     dst,
		})
	})

	// ====================================
	// Multiple File Upload Example
	// ====================================
	router.Post("/upload-multiple", func(ctx *microweb.Context) {
		form, err := ctx.MultipartForm()
		if err != nil {
			ctx.Status(http.StatusBadRequest)
			ctx.Json(map[string]string{"error": "Failed to parse form"})
			return
		}

		files := form.File["files"]
		uploadedFiles := []string{}

		for _, fileHeader := range files {
			file, err := fileHeader.Open()
			if err != nil {
				continue
			}

			dst := "./uploads/" + fileHeader.Filename
			if err := ctx.SaveUploadedFile(file, fileHeader, dst); err != nil {
				file.Close()
				continue
			}
			file.Close()
			uploadedFiles = append(uploadedFiles, fileHeader.Filename)
		}

		ctx.Json(map[string]interface{}{
			"message": "Files uploaded successfully",
			"files":   uploadedFiles,
			"count":   len(uploadedFiles),
		})
	})

	// ====================================
	// Request Context Cancellation Example
	// ====================================
	router.Get("/long-task", func(ctx *microweb.Context) {
		reqCtx := ctx.Context()

		// Simulate a long-running task
		select {
		case <-time.After(5 * time.Second):
			ctx.String("Task completed!")
		case <-reqCtx.Done():
			log.Println("Request cancelled by client")
			ctx.Status(http.StatusRequestTimeout)
			ctx.String("Request cancelled")
		}
	})

	// ====================================
	// Panic Route (to test panic recovery)
	// ====================================
	router.Get("/panic", func(ctx *microweb.Context) {
		panic("Something went terribly wrong!")
	})

	// ====================================
	// Route Groups
	// ====================================
	api := router.Group("/api")

	// API middleware
	api.Use(func(ctx *microweb.Context) bool {
		log.Println("API middleware executed")
		return true
	})

	api.Get("/users", func(ctx *microweb.Context) {
		ctx.Json(map[string]interface{}{
			"users": []string{"Alice", "Bob", "Charlie"},
		})
	})

	api.Post("/users", func(ctx *microweb.Context) {
		var user map[string]string
		if err := ctx.Parse(&user); err != nil {
			ctx.Status(http.StatusBadRequest)
			ctx.Json(map[string]string{"error": "Invalid JSON"})
			return
		}
		ctx.Status(http.StatusCreated)
		ctx.Json(map[string]interface{}{
			"message": "User created",
			"user":    user,
		})
	})

	// Nested groups
	v1 := api.Group("/v1")
	v1.Get("/info", func(ctx *microweb.Context) {
		ctx.Json(map[string]string{
			"version": "1.0.0",
			"api":     "v1",
		})
	})

	// ====================================
	// Start Server
	// ====================================
	log.Println("Server starting on :8080")
	if err := router.Listen(8080); err != nil {
		log.Fatal(err)
	}
}
