# Microweb

A lightweight, fast, and easy-to-use HTTP web framework for Go, inspired by Express.js.

## Features

- 🚀 **Minimal and Fast** - Built on Go's standard `net/http` package
- 🛣️ **Route Groups** - Organize routes with prefixes and shared middleware
- 🔧 **Middleware Support** - Pre and post-request middleware
- 📁 **Static File Serving** - Serve static files with or without prefix
- 🍪 **Cookie Support** - Easy cookie management
- 📤 **File Upload** - Single and multiple file upload support
- 🔄 **Redirect Helpers** - Simple redirect methods
- 🛡️ **Panic Recovery** - Built-in panic recovery with custom handlers
- 🌐 **CORS Support** - Built-in CORS middleware
- 🔍 **Context Management** - Request context cancellation support
- 🎯 **Custom Error Handlers** - 404 and 405 custom handlers
- 📝 **Template Rendering** - HTML template support
- 🎨 **JSON Support** - Easy JSON encoding/decoding with proper Content-Type
- 🔌 **WebSocket Support** - Real-time bidirectional communication with event-based API

## Installation

```bash
go get github.com/sfi2k7/microweb
```

## Quick Start

```go
package main

import (
    "github.com/sfi2k7/microweb"
    "log"
)

func main() {
    router := microweb.New()
    
    router.Get("/", func(ctx *microweb.Context) {
        ctx.String("Hello, World!")
    })
    
    log.Fatal(router.Listen(8080))
}
```

## Basic Routing

Microweb supports all standard HTTP methods:

```go
router := microweb.New()

router.Get("/users", getUsers)
router.Post("/users", createUser)
router.Put("/users/{id}", updateUser)
router.Delete("/users/{id}", deleteUser)
router.Patch("/users/{id}", patchUser)
router.Options("/users", optionsHandler)
router.Head("/users", headHandler)
```

### Path Parameters

```go
router.Get("/hello/{name}", func(ctx *microweb.Context) {
    name := ctx.Param("name")
    ctx.Json(map[string]string{
        "message": "Hello, " + name,
    })
})
```

### Query Parameters

```go
router.Get("/search", func(ctx *microweb.Context) {
    query := ctx.Query("q")
    ctx.Json(map[string]string{
        "query": query,
    })
})
```

## Context Methods

### Response Methods

```go
// Send JSON response (sets Content-Type: application/json)
ctx.Json(map[string]string{"message": "success"})

// Send plain text (sets Content-Type: text/plain; charset=utf-8)
ctx.String("Hello, World!")

// Render HTML template
ctx.View("index.html", data)

// Set status code
ctx.Status(http.StatusCreated)

// Redirect
ctx.Redirect("/login", http.StatusFound)
```

### Request Methods

```go
// Get path parameter
name := ctx.Param("id")

// Get query parameter
search := ctx.Query("q")

// Get header
auth := ctx.Header("Authorization")

// Parse JSON body
var user User
ctx.Parse(&user)

// Get raw body
body, err := ctx.Body()

// Get form value
email := ctx.FormValue("email")
```

### Cookie Methods

```go
// Set cookie
ctx.SetCookie(&http.Cookie{
    Name:     "session",
    Value:    "abc123",
    Path:     "/",
    MaxAge:   3600,
    HttpOnly: true,
})

// Get cookie
cookie, err := ctx.Cookie("session")
```

### State Management

```go
// Set state (for passing data between middlewares)
ctx.Set("user", user)

// Get state
user := ctx.Get("user")
```

### File Upload

```go
// Single file upload
router.Post("/upload", func(ctx *microweb.Context) {
    file, header, err := ctx.FormFile("file")
    if err != nil {
        ctx.Status(http.StatusBadRequest)
        ctx.Json(map[string]string{"error": "No file uploaded"})
        return
    }
    
    // Save file
    dst := "./uploads/" + header.Filename
    if err := ctx.SaveUploadedFile(file, header, dst); err != nil {
        ctx.Status(http.StatusInternalServerError)
        ctx.Json(map[string]string{"error": "Failed to save file"})
        return
    }
    
    ctx.Json(map[string]string{"message": "File uploaded successfully"})
})

// Multiple file upload
router.Post("/upload-multiple", func(ctx *microweb.Context) {
    form, err := ctx.MultipartForm()
    if err != nil {
        ctx.Status(http.StatusBadRequest)
        return
    }
    
    files := form.File["files"]
    for _, fileHeader := range files {
        file, _ := fileHeader.Open()
        ctx.SaveUploadedFile(file, fileHeader, "./uploads/"+fileHeader.Filename)
        file.Close()
    }
    
    ctx.Json(map[string]string{"message": "Files uploaded"})
})
```

### Request Context

```go
// Get request context (for cancellation, timeouts, etc.)
router.Get("/long-task", func(ctx *microweb.Context) {
    reqCtx := ctx.Context()
    
    select {
    case <-time.After(5 * time.Second):
        ctx.String("Task completed!")
    case <-reqCtx.Done():
        ctx.String("Request cancelled")
    }
})
```

## Middleware

### Using Middleware

```go
router := microweb.New()

// Pre-middleware (runs before handlers)
router.Use(loggingMiddleware, authMiddleware)

// Post-middleware (runs after handlers)
router.UseAfter(cleanupMiddleware)

// Middleware function signature
func loggingMiddleware(ctx *microweb.Context) bool {
    log.Printf("%s %s", ctx.Method, ctx.R.URL.Path)
    return true // continue to next middleware/handler
}

func authMiddleware(ctx *microweb.Context) bool {
    token := ctx.Header("Authorization")
    if token == "" {
        ctx.Status(http.StatusUnauthorized)
        ctx.Json(map[string]string{"error": "Unauthorized"})
        return false // stop execution
    }
    return true
}
```

### CORS Middleware

```go
router.Use(microweb.CORS(
    "*",                                    // Allow-Origin
    "GET,POST,PUT,DELETE,OPTIONS",         // Allow-Methods
    "Content-Type,Authorization",          // Allow-Headers
))
```

## Route Groups

### Basic Groups

```go
router := microweb.New()

// Create a group with prefix
api := router.Group("/api")

// Add middleware to group
api.Use(func(ctx *microweb.Context) bool {
    log.Println("API middleware")
    return true
})

// Add routes to group
api.Get("/users", getUsers)       // /api/users
api.Post("/users", createUser)    // /api/users

// Nested groups
v1 := api.Group("/v1")
v1.Get("/info", getInfo)          // /api/v1/info

v2 := api.Group("/v2")
v2.Get("/info", getInfoV2)        // /api/v2/info
```

### Group Features

#### Any Method Handler

```go
// Handle all HTTP methods on a path
api.Any("/webhook", func(ctx *microweb.Context) {
    ctx.Json(map[string]string{
        "method": ctx.Method,
        "path": ctx.R.URL.Path,
    })
})
```

#### Match Specific Methods

```go
// Handle only specific methods
api.Match([]string{"GET", "POST"}, "/contact", func(ctx *microweb.Context) {
    if ctx.Method == "GET" {
        ctx.String("Contact form")
    } else {
        ctx.String("Form submitted")
    }
})
```

#### Route-Specific Middleware

```go
// Apply middleware to a specific route only
group := router.Group("/special")

rateLimitMiddleware := func(ctx *microweb.Context) bool {
    // Rate limiting logic
    return true
}

group.Get("/limited", group.UseOnly(
    func(ctx *microweb.Context) {
        ctx.String("This route has rate limiting")
    },
    rateLimitMiddleware,
))
```

#### Static Files in Groups

```go
// Serve static files at the group's prefix
assets := router.Group("/assets")
assets.Static("./public/assets") // Serves ./public/assets at /assets/*
```

#### Route Debugging

```go
// Get all routes in a group
api := router.Group("/api")
api.Get("/users", getUsers)
api.Post("/users", createUser)

// Get direct routes only (not including children)
routes := api.Routes()

// Get all routes including child groups
allRoutes := api.AllRoutes()

// Get group prefix
prefix := api.Prefix() // returns "/api"

// Get all routes from router (includes all groups)
allRoutes := router.Routes()
```

#### Middleware Execution Order

Parent group middlewares execute **before** child group middlewares:

```go
api := router.Group("/api")
api.Use(middleware1) // Runs first

v1 := api.Group("/v1")
v1.Use(middleware2)  // Runs second

v1.Get("/users", handler) // Runs third

// Execution order: middleware1 → middleware2 → handler
```

#### Example: API Versioning with Groups

```go
router := microweb.New()

api := router.Group("/api")
api.Use(authMiddleware) // All API routes require auth

// V1 API
v1 := api.Group("/v1")
v1.Use(func(ctx *microweb.Context) bool {
    ctx.Set("version", "v1")
    return true
})
v1.Get("/users", getUsersV1)      // /api/v1/users
v1.Get("/products", getProductsV1) // /api/v1/products

// V2 API with different structure
v2 := api.Group("/v2")
v2.Use(func(ctx *microweb.Context) bool {
    ctx.Set("version", "v2")
    return true
})
v2.Get("/users", getUsersV2)      // /api/v2/users
v2.Any("/products", handleProducts) // /api/v2/products (all methods)

// Admin section
admin := v2.Group("/admin")
admin.Use(adminAuthMiddleware)
admin.Get("/stats", getStats)     // /api/v2/admin/stats
```

## Static File Serving

### Root Level Static Files

```go
// Serve static files from ./public directory at root level
// Files must exist to be served (prevents directory listing)
router.Static("./public")
// Now /favicon.ico, /robots.txt serve from ./public/
```

### Prefix-Based Static Files

```go
// Serve static files from ./assets under /static prefix
router.StaticWithPrefix("/static", "./assets")
// Now /static/css/style.css serves ./assets/css/style.css
```

### Both Together

```go
router.Static("./public")                      // for root files
router.StaticWithPrefix("/static", "./assets") // for /static/* files
```

## Error Handling

### Panic Recovery

Microweb automatically recovers from panics in handlers:

```go
router := microweb.New()

// Default panic handler logs and returns 500
router.Get("/panic", func(ctx *microweb.Context) {
    panic("Something went wrong!")
})

// Custom panic handler
router.SetPanicHandler(func(ctx *microweb.Context, err any) {
    log.Printf("Panic: %v", err)
    ctx.Status(http.StatusInternalServerError)
    ctx.Json(map[string]string{
        "error": "Internal server error",
        "message": fmt.Sprintf("%v", err),
    })
})
```

### Custom 404 Handler

```go
router.SetNotFoundHandler(func(ctx *microweb.Context) {
    ctx.Status(http.StatusNotFound)
    ctx.Json(map[string]string{
        "error": "Route not found",
        "path": ctx.R.URL.Path,
    })
})
```

### Custom 405 Method Not Allowed Handler

```go
router.SetMethodNotAllowedHandler(func(ctx *microweb.Context) {
    ctx.Status(http.StatusMethodNotAllowed)
    ctx.Json(map[string]string{
        "error": "Method not allowed",
        "method": ctx.Method,
    })
})
```

## Complete Example

```go
package main

import (
    "log"
    "net/http"
    "github.com/sfi2k7/microweb"
)

func main() {
    router := microweb.New()
    
    // CORS
    router.Use(microweb.CORS("*", "GET,POST,PUT,DELETE,OPTIONS", "Content-Type,Authorization"))
    
    // Static files
    router.Static("./public")
    router.StaticWithPrefix("/static", "./assets")
    
    // Error handlers
    router.SetNotFoundHandler(func(ctx *microweb.Context) {
        ctx.Status(http.StatusNotFound)
        ctx.Json(map[string]string{"error": "Not found"})
    })
    
    router.SetPanicHandler(func(ctx *microweb.Context, err any) {
        ctx.Status(http.StatusInternalServerError)
        ctx.Json(map[string]string{"error": "Server error"})
    })
    
    // Routes
    router.Get("/", func(ctx *microweb.Context) {
        ctx.String("Welcome!")
    })
    
    router.Get("/hello/{name}", func(ctx *microweb.Context) {
        ctx.Json(map[string]string{
            "message": "Hello, " + ctx.Param("name"),
        })
    })
    
    // API Group
    api := router.Group("/api")
    api.Use(authMiddleware)
    
    api.Get("/users", getUsers)
    api.Post("/users", createUser)
    
    // Start server
    log.Println("Server running on :8080")
    log.Fatal(router.Listen(8080))
}

func authMiddleware(ctx *microweb.Context) bool {
    token := ctx.Header("Authorization")
    if token == "" {
        ctx.Status(http.StatusUnauthorized)
        ctx.Json(map[string]string{"error": "Unauthorized"})
        return false
    }
    return true
}

func getUsers(ctx *microweb.Context) {
    ctx.Json([]string{"Alice", "Bob", "Charlie"})
}

func createUser(ctx *microweb.Context) {
    var user map[string]string
    if err := ctx.Parse(&user); err != nil {
        ctx.Status(http.StatusBadRequest)
        ctx.Json(map[string]string{"error": "Invalid JSON"})
        return
    }
    ctx.Status(http.StatusCreated)
    ctx.Json(user)
}
```

## WebSocket Support

Microweb includes built-in WebSocket support with a simple, event-based API.

### Basic WebSocket Setup

```go
router := microweb.New()

router.Ws("/ws", func(ctx *microweb.ClientContext) *microweb.WsData {
    // Lifecycle hooks
    ctx.On("open", func(c *microweb.ClientContext) {
        log.Printf("Client connected: %s", c.Id)
    })
    
    ctx.On("close", func(c *microweb.ClientContext) {
        log.Printf("Client disconnected: %s", c.Id)
    })
    
    ctx.On("error", func(c *microweb.ClientContext) {
        log.Printf("Error: %v", c.Data.Get("error"))
    })
    
    // Handle messages
    cmd := ctx.Data.String("cmd")
    
    switch cmd {
    case "ping":
        return microweb.NewWsDataFromMap(map[string]interface{}{
            "type": "pong",
        })
    case "echo":
        return microweb.NewWsDataFromMap(map[string]interface{}{
            "message": ctx.Data.String("message"),
        })
    }
    
    return nil // No reply
})
```

### Client Context

**ClientContext** is passed to your WebSocket handler for each message:

- `ctx.Id` - Unique client ID (UUID without dashes)
- `ctx.Data` - Parsed JSON message with type-safe getters
- `ctx.Send(data)` - Send message to this client
- `ctx.Close()` - Close this connection
- `ctx.On(event, handler)` - Register lifecycle event handlers

### WsData Methods

Access incoming JSON data with type-safe getters:

```go
cmd := ctx.Data.String("cmd")        // Get string
count := ctx.Data.Int("count")       // Get int
price := ctx.Data.Float("price")     // Get float64
active := ctx.Data.Bool("active")    // Get bool
raw := ctx.Data.Get("key")           // Get raw interface{}
exists := ctx.Data.Has("key")        // Check if key exists

// Create response data
reply := microweb.NewWsDataFromMap(map[string]interface{}{
    "status": "ok",
    "count": 42,
})
```

### Global Hub

Access the WebSocket hub from anywhere in your application:

```go
// Send to specific client
microweb.Hub.Send(clientId, map[string]string{
    "type": "notification",
    "message": "Hello!",
})

// Broadcast to all clients
microweb.Hub.Broadcast(map[string]string{
    "type": "announcement",
    "message": "Server maintenance in 5 minutes",
})

// Close specific connection
microweb.Hub.Close(clientId)

// Get connected clients count
count := microweb.Hub.Count()
```

### Send from HTTP Handlers

```go
// Notify WebSocket client from HTTP endpoint
router.Post("/notify/{clientId}", func(ctx *microweb.Context) {
    clientId := ctx.Param("clientId")
    message := ctx.FormValue("message")
    
    microweb.Hub.Send(clientId, map[string]string{
        "notification": message,
    })
    
    ctx.Json(map[string]bool{"sent": true})
})
```

### Lifecycle Events

- `"open"` - Client connected
- `"close"` - Client disconnected
- `"error"` - Error occurred (excludes idle/read timeouts)

### Complete WebSocket Example

```go
router := microweb.New()

router.Ws("/ws", func(ctx *microweb.ClientContext) *microweb.WsData {
    ctx.On("open", func(c *microweb.ClientContext) {
        c.Send(map[string]string{
            "type": "welcome",
            "clientId": c.Id,
        })
    })
    
    cmd := ctx.Data.String("cmd")
    
    switch cmd {
    case "broadcast":
        message := ctx.Data.String("message")
        microweb.Hub.Broadcast(map[string]interface{}{
            "from": ctx.Id,
            "message": message,
        })
        return nil
        
    case "stats":
        return microweb.NewWsDataFromMap(map[string]interface{}{
            "clients": microweb.Hub.Count(),
            "yourId": ctx.Id,
        })
    }
    
    return nil
})
```

### Configuration

Customize WebSocket behavior (optional):

```go
config := &microweb.WsConfig{
    PingInterval:    30 * time.Second,
    PongWait:        60 * time.Second,
    WriteWait:       10 * time.Second,
    MaxMessageSize:  512 * 1024, // 512 KB
    ReadBufferSize:  1024,
    WriteBufferSize: 1024,
}

// Apply when initializing (custom hub setup)
microweb.Hub = microweb.NewWsHub(config)
```

## Requirements

- Go 1.24.7 or higher

## License

MIT

## Contributing

Contributions are welcome! Please feel free to submit a Pull Request.