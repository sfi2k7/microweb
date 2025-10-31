package microweb

import (
	"net/http"
	"path/filepath"
)

type Group struct {
	r          *Router
	prefix     string
	middleware []MiddleWare
	parent     *Group
	children   []*Group
	routes     []string // track registered routes
}

func (g *Group) Group(prefix string) *Group {
	child := &Group{
		r:          g.r,
		parent:     g,
		prefix:     g.prefix + prefix,
		middleware: []MiddleWare{},
		children:   []*Group{},
		routes:     []string{},
	}
	g.children = append(g.children, child)
	return child
}

func (g *Group) runMiddlewares(ctx *Context) bool {
	// Run parent middlewares first (fixed order)
	if g.parent != nil {
		if !g.parent.runMiddlewares(ctx) {
			return false
		}
	}

	// Then run this group's middlewares
	for _, m := range g.middleware {
		if !m(ctx) {
			return false
		}
	}

	return true
}

func (g *Group) middle(h Handler) Handler {
	return func(ctx *Context) {
		if !g.runMiddlewares(ctx) {
			return
		}

		h(ctx)
	}
}

func (g *Group) Use(middlewares ...MiddleWare) {
	g.middleware = append(g.middleware, middlewares...)
}

// UseOnly applies middleware to a specific handler without adding to group
func (g *Group) UseOnly(handler Handler, middlewares ...MiddleWare) Handler {
	return func(ctx *Context) {
		for _, m := range middlewares {
			if !m(ctx) {
				return
			}
		}
		handler(ctx)
	}
}

func (g *Group) Get(path string, handler Handler) {
	fullPath := filepath.Join(g.prefix, path)
	g.routes = append(g.routes, "GET "+fullPath)
	g.r.Get(fullPath, g.middle(handler))
}

func (g *Group) Post(path string, handler Handler) {
	fullPath := filepath.Join(g.prefix, path)
	g.routes = append(g.routes, "POST "+fullPath)
	g.r.Post(fullPath, g.middle(handler))
}

func (g *Group) Put(path string, handler Handler) {
	fullPath := filepath.Join(g.prefix, path)
	g.routes = append(g.routes, "PUT "+fullPath)
	g.r.Put(fullPath, g.middle(handler))
}

func (g *Group) Delete(path string, handler Handler) {
	fullPath := filepath.Join(g.prefix, path)
	g.routes = append(g.routes, "DELETE "+fullPath)
	g.r.Delete(fullPath, g.middle(handler))
}

func (g *Group) Patch(path string, handler Handler) {
	fullPath := filepath.Join(g.prefix, path)
	g.routes = append(g.routes, "PATCH "+fullPath)
	g.r.Patch(fullPath, g.middle(handler))
}

func (g *Group) Options(path string, handler Handler) {
	fullPath := filepath.Join(g.prefix, path)
	g.routes = append(g.routes, "OPTIONS "+fullPath)
	g.r.Options(fullPath, g.middle(handler))
}

func (g *Group) Head(path string, handler Handler) {
	fullPath := filepath.Join(g.prefix, path)
	g.routes = append(g.routes, "HEAD "+fullPath)
	g.r.Head(fullPath, g.middle(handler))
}

// Any registers a handler for all HTTP methods
func (g *Group) Any(path string, handler Handler) {
	g.Get(path, handler)
	g.Post(path, handler)
	g.Put(path, handler)
	g.Delete(path, handler)
	g.Patch(path, handler)
	g.Options(path, handler)
	g.Head(path, handler)
}

// Match registers a handler for specific HTTP methods
func (g *Group) Match(methods []string, path string, handler Handler) {
	fullPath := filepath.Join(g.prefix, path)
	wrappedHandler := g.middle(handler)

	for _, method := range methods {
		g.routes = append(g.routes, method+" "+fullPath)
		switch method {
		case http.MethodGet:
			g.r.Get(fullPath, wrappedHandler)
		case http.MethodPost:
			g.r.Post(fullPath, wrappedHandler)
		case http.MethodPut:
			g.r.Put(fullPath, wrappedHandler)
		case http.MethodDelete:
			g.r.Delete(fullPath, wrappedHandler)
		case http.MethodPatch:
			g.r.Patch(fullPath, wrappedHandler)
		case http.MethodOptions:
			g.r.Options(fullPath, wrappedHandler)
		case http.MethodHead:
			g.r.Head(fullPath, wrappedHandler)
		}
	}
}

// Static serves static files at the group's prefix
func (g *Group) Static(path string) {
	g.r.StaticWithPrefix(g.prefix, path)
}

// Routes returns all routes registered in this group (not including children)
func (g *Group) Routes() []string {
	return g.routes
}

// AllRoutes returns all routes including child groups
func (g *Group) AllRoutes() []string {
	routes := make([]string, len(g.routes))
	copy(routes, g.routes)

	for _, child := range g.children {
		routes = append(routes, child.AllRoutes()...)
	}

	return routes
}

// Prefix returns the group's prefix
func (g *Group) Prefix() string {
	return g.prefix
}
