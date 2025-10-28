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

type Router struct {
	staticisset    bool
	staticPath     string
	premiddleware  []MiddleWare
	postmiddleware []MiddleWare
	endpoints      map[string]map[string]Handler
	count          atomic.Int64
	mux            *http.ServeMux
	staticprefix   string
	groups         []*Group
}

func New() *Router {
	return &Router{
		endpoints: make(map[string]map[string]Handler),
		count:     atomic.Int64{},
		mux:       http.NewServeMux(),
	}
}

func (r *Router) Group(prefix string) *Group {
	g := &Group{
		r:          r,
		prefix:     prefix,
		middleware: []MiddleWare{},
		children:   []*Group{},
		parent:     nil,
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

func (mw *Router) StaticWithPrefix(prefix, path string) {
	if !strings.HasSuffix(prefix, "/") {
		prefix += "/"
	}

	if !strings.HasPrefix(path, "/") {
		prefix += "/" + prefix
	}

	mw.staticprefix = prefix
}

func (mw *Router) StaticPath(path string) {
	mw.mux.Handle("GET /", http.StripPrefix("/", http.FileServer(http.Dir(path))))
}

func (mw *Router) fileExists(filepath string) bool {
	info, err := os.Stat(filepath)
	if os.IsNotExist(err) {
		return false
	}
	return !info.IsDir()
}

func (mw *Router) Get(path string, handler func(*Context)) {
	mw.addroute(path, http.MethodGet, handler)
}

func (mw *Router) Post(path string, handler func(*Context)) {
	mw.addroute(path, http.MethodPost, handler)
}

func (mw *Router) Put(path string, handler func(*Context)) {
	mw.addroute(path, http.MethodPut, handler)
}

func (mw *Router) Delete(path string, handler func(*Context)) {
	mw.addroute(path, http.MethodDelete, handler)
}

func (mw *Router) Head(path string, handler func(*Context)) {
	mw.addroute(path, http.MethodHead, handler)
}

func (mw *Router) Options(path string, handler func(*Context)) {
	mw.addroute(path, http.MethodOptions, handler)
}

func (mw *Router) Patch(path string, handler func(*Context)) {
	mw.addroute(path, http.MethodPatch, handler)
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

	// Check if this is a static file request
	// if mw.staticPath != "" {

	// 	// Check for /static/ prefix first
	// 	if mw.staticprefix != "" && strings.HasPrefix(r.URL.Path, mw.staticprefix) {
	// 		fileServer := http.StripPrefix(mw.staticprefix, http.FileServer(http.Dir(mw.staticPath)))
	// 		fileServer.ServeHTTP(w, r)
	// 		return
	// 	}

	// 	// fmt.Println("check if file exists", mw.staticPath+r.URL.Path, mw.fileExists(mw.staticPath+r.URL.Path))
	// 	// Check for root-level static files based on file existence
	// 	if mw.fileExists(mw.staticPath + r.URL.Path) {
	// 		fileServer := http.FileServer(http.Dir(mw.staticPath))
	// 		fileServer.ServeHTTP(w, r)
	// 		return
	// 	}
	// }

	mw.count.Store(mw.count.Add(1))

	start := time.Now()

	defer func() {
		log.Printf("%s %s %s #%d", r.Method, r.URL.Path, time.Since(start), mw.count.Load())
	}()

	mw.mux.ServeHTTP(w, r)
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
