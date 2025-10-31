package microweb

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"sync/atomic"
	"syscall"
	"time"
)

type MiddleWare func(c *Context) bool
type Handler func(*Context)
type PanicHandler func(c *Context, err any)

type Router struct {
	staticisset             bool
	staticPath              string
	premiddleware           []MiddleWare
	postmiddleware          []MiddleWare
	endpoints               map[string]map[string]Handler
	count                   atomic.Int64
	mux                     *http.ServeMux
	staticprefix            string
	groups                  []*Group
	panicHandler            PanicHandler
	notFoundHandler         Handler
	methodNotAllowedHandler Handler
	routes                  []string
}

func New() *Router {
	return &Router{
		endpoints: make(map[string]map[string]Handler),
		count:     atomic.Int64{},
		mux:       http.NewServeMux(),
		routes:    []string{},
	}
}

func (r *Router) Group(prefix string) *Group {
	g := &Group{
		r:          r,
		prefix:     prefix,
		middleware: []MiddleWare{},
		children:   []*Group{},
		parent:     nil,
		routes:     []string{},
	}

	r.groups = append(r.groups, g)
	return g
}

func (r *Router) Use(middlewares ...MiddleWare) {
	r.premiddleware = append(r.premiddleware, middlewares...)
}

func (r *Router) UseAfter(middlewares ...MiddleWare) {
	r.postmiddleware = append(r.postmiddleware, middlewares...)
}

func (r *Router) SetPanicHandler(handler PanicHandler) {
	r.panicHandler = handler
}

func (r *Router) SetNotFoundHandler(handler Handler) {
	r.notFoundHandler = handler
}

func (r *Router) SetMethodNotAllowedHandler(handler Handler) {
	r.methodNotAllowedHandler = handler
}

// CORS middleware helper
func CORS(allowOrigin, allowMethods, allowHeaders string) MiddleWare {
	return func(c *Context) bool {
		if allowOrigin != "" {
			c.W.Header().Set("Access-Control-Allow-Origin", allowOrigin)
		}
		if allowMethods != "" {
			c.W.Header().Set("Access-Control-Allow-Methods", allowMethods)
		}
		if allowHeaders != "" {
			c.W.Header().Set("Access-Control-Allow-Headers", allowHeaders)
		}
		c.W.Header().Set("Access-Control-Allow-Credentials", "true")

		// Handle preflight OPTIONS request
		if c.Method == "OPTIONS" {
			c.W.WriteHeader(http.StatusNoContent)
			return false
		}
		return true
	}
}

func (mw *Router) Static(path string) {
	mw.staticPath = path
	mw.staticisset = true
}

func (mw *Router) StaticWithPrefix(prefix, path string) {
	// Ensure prefix starts and ends with /
	if !strings.HasPrefix(prefix, "/") {
		prefix = "/" + prefix
	}
	if !strings.HasSuffix(prefix, "/") {
		prefix += "/"
	}

	mw.staticprefix = prefix
	mw.staticPath = path
	mw.staticisset = true
}

func (mw *Router) fileExists(filepath string) bool {
	info, err := os.Stat(filepath)
	if os.IsNotExist(err) {
		return false
	}
	return err == nil && !info.IsDir()
}

func (mw *Router) Get(path string, handler func(*Context)) {
	mw.routes = append(mw.routes, "GET "+path)
	mw.addroute(path, http.MethodGet, handler)
}

func (mw *Router) Post(path string, handler func(*Context)) {
	mw.routes = append(mw.routes, "POST "+path)
	mw.addroute(path, http.MethodPost, handler)
}

func (mw *Router) Put(path string, handler func(*Context)) {
	mw.routes = append(mw.routes, "PUT "+path)
	mw.addroute(path, http.MethodPut, handler)
}

func (mw *Router) Delete(path string, handler func(*Context)) {
	mw.routes = append(mw.routes, "DELETE "+path)
	mw.addroute(path, http.MethodDelete, handler)
}

func (mw *Router) Head(path string, handler func(*Context)) {
	mw.routes = append(mw.routes, "HEAD "+path)
	mw.addroute(path, http.MethodHead, handler)
}

func (mw *Router) Options(path string, handler func(*Context)) {
	mw.routes = append(mw.routes, "OPTIONS "+path)
	mw.addroute(path, http.MethodOptions, handler)
}

func (mw *Router) Patch(path string, handler func(*Context)) {
	mw.routes = append(mw.routes, "PATCH "+path)
	mw.addroute(path, http.MethodPatch, handler)
}

// Any registers a handler for all HTTP methods
func (mw *Router) Any(path string, handler Handler) {
	mw.Get(path, handler)
	mw.Post(path, handler)
	mw.Put(path, handler)
	mw.Delete(path, handler)
	mw.Patch(path, handler)
	mw.Options(path, handler)
	mw.Head(path, handler)
}

// Match registers a handler for specific HTTP methods
func (mw *Router) Match(methods []string, path string, handler Handler) {
	for _, method := range methods {
		mw.routes = append(mw.routes, method+" "+path)
		switch method {
		case http.MethodGet:
			mw.Get(path, handler)
		case http.MethodPost:
			mw.Post(path, handler)
		case http.MethodPut:
			mw.Put(path, handler)
		case http.MethodDelete:
			mw.Delete(path, handler)
		case http.MethodPatch:
			mw.Patch(path, handler)
		case http.MethodOptions:
			mw.Options(path, handler)
		case http.MethodHead:
			mw.Head(path, handler)
		}
	}
}

// Routes returns all registered routes
func (mw *Router) Routes() []string {
	routes := make([]string, len(mw.routes))
	copy(routes, mw.routes)

	// Include routes from all groups
	for _, g := range mw.groups {
		routes = append(routes, g.AllRoutes()...)
	}

	return routes
}

func (mw *Router) addroute(path, method string, handler Handler) error {
	mw.mux.HandleFunc(method+" "+path, mw.middle(handler))
	return nil
}

func (mw *Router) runMiddlewares(ctx *Context) bool {

	for _, m := range mw.premiddleware {
		if !m(ctx) {
			return false
		}
	}

	return true
}

func (mw *Router) middle(fn func(*Context)) http.HandlerFunc {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {

		ctx := &Context{R: r, W: w, Method: r.Method, state: make(map[string]any)}

		// Panic recovery
		defer func() {
			if err := recover(); err != nil {
				log.Printf("PANIC: %v", err)
				if mw.panicHandler != nil {
					mw.panicHandler(ctx, err)
				} else {
					ctx.W.WriteHeader(http.StatusInternalServerError)
					ctx.W.Write([]byte("Internal Server Error"))
				}
			}
		}()

		if !mw.runMiddlewares(ctx) {
			return
		}

		fn(ctx)

		for _, middleware := range mw.postmiddleware {
			if next := middleware(ctx); !next {
				return
			}
		}
	})
}

func (mw *Router) ServeHTTP(w http.ResponseWriter, r *http.Request) {

	// Check if static file serving is enabled
	if mw.staticisset && mw.staticPath != "" {
		// Check for /prefix/ based static files first
		if mw.staticprefix != "" && strings.HasPrefix(r.URL.Path, mw.staticprefix) {
			fileServer := http.StripPrefix(mw.staticprefix, http.FileServer(http.Dir(mw.staticPath)))
			fileServer.ServeHTTP(w, r)
			return
		}

		// Check for root-level static files based on file existence
		// Only if no prefix is set or path doesn't match prefix
		if mw.staticprefix == "" && mw.fileExists(mw.staticPath+r.URL.Path) {
			fileServer := http.FileServer(http.Dir(mw.staticPath))
			fileServer.ServeHTTP(w, r)
			return
		}
	}

	mw.count.Add(1)

	start := time.Now()

	defer func() {
		log.Printf("%s %s %s #%d", r.Method, r.URL.Path, time.Since(start), mw.count.Load())
	}()

	// Check if this is a WebSocket upgrade request
	isWebSocket := r.Header.Get("Upgrade") == "websocket"

	if isWebSocket {
		// Don't wrap for WebSocket - needs Hijacker interface
		mw.mux.ServeHTTP(w, r)
		return
	}

	// Create a custom response writer to capture status code
	crw := &customResponseWriter{ResponseWriter: w, statusCode: http.StatusOK}
	mw.mux.ServeHTTP(crw, r)

	// Handle 404 and 405 with custom handlers
	if crw.statusCode == http.StatusNotFound && mw.notFoundHandler != nil {
		ctx := &Context{R: r, W: w, Method: r.Method, state: make(map[string]any)}
		mw.notFoundHandler(ctx)
	} else if crw.statusCode == http.StatusMethodNotAllowed && mw.methodNotAllowedHandler != nil {
		ctx := &Context{R: r, W: w, Method: r.Method, state: make(map[string]any)}
		mw.methodNotAllowedHandler(ctx)
	}
}

// customResponseWriter wraps http.ResponseWriter to capture status code
type customResponseWriter struct {
	http.ResponseWriter
	statusCode int
}

func (crw *customResponseWriter) WriteHeader(code int) {
	crw.statusCode = code
	crw.ResponseWriter.WriteHeader(code)
}

func (mw *Router) Listen(port int) error {
	ex := make(chan os.Signal, 2)
	signal.Notify(ex, os.Interrupt, syscall.SIGTERM)
	defer signal.Stop(ex)

	go func() {
		<-ex
		os.Exit(0)
	}()

	return http.ListenAndServe(fmt.Sprintf(":%d", port), mw)
}
