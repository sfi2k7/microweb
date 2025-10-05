package microweb

import (
	"encoding/json"
	"errors"
	"fmt"
	"html/template"
	"io"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"sync/atomic"
	"syscall"
	"time"
)

type Context struct {
	R      *http.Request
	W      http.ResponseWriter
	Method string
}

type MiddleWare func(c *Context) bool
type Handler func(*Context)

func (tc *Context) Json(v any) error {
	return json.NewEncoder(tc.W).Encode(v)
}

func (tc *Context) Query(key string) string {
	return tc.R.URL.Query().Get(key)
}

func (tc *Context) Status(status int) {
	tc.W.WriteHeader(status)
}

func (tc *Context) StatusOk(status int) {
	tc.W.WriteHeader(http.StatusOK)
}

func (tc *Context) StatusServerError(status int) {
	tc.W.WriteHeader(http.StatusInternalServerError)
}

func (tc *Context) StatusBadRequest(status int) {
	tc.W.WriteHeader(http.StatusBadRequest)
}

func (tc *Context) Parse(target any) error {
	body, err := io.ReadAll(tc.R.Body)
	if err != nil {
		return err
	}
	defer tc.R.Body.Close()

	return json.Unmarshal(body, target)
}

func (tc *Context) Body() ([]byte, error) {
	body, err := io.ReadAll(tc.R.Body)
	if err != nil {
		return nil, err
	}

	return body, nil
}

func (tc *Context) String(str string) error {
	tc.W.WriteHeader(http.StatusOK)
	_, err := fmt.Fprintf(tc.W, "%s", str)
	return err
}

type MicroWeb struct {
	staticisset    bool
	premiddleware  []MiddleWare
	postmiddleware []MiddleWare
	endpoints      map[string]map[string]Handler
	count          atomic.Int64
}

func New() *MicroWeb {
	return &MicroWeb{
		endpoints: make(map[string]map[string]Handler),
		count:     atomic.Int64{},
	}
}

func (mw *MicroWeb) UseBefore(middlewares ...MiddleWare) {
	mw.premiddleware = append(mw.premiddleware, middlewares...)
}

func (mw *MicroWeb) UseAfter(middlewares ...MiddleWare) {
	mw.postmiddleware = append(mw.postmiddleware, middlewares...)
}

// func (mw *MicroWeb) middle(fn func(*Context)) http.HandlerFunc {
// 	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
// 		ctx := &Context{R: r, W: w, Method: r.Method}

// 		for _, middleware := range mw.premiddleware {
// 			if next := middleware(ctx); !next {
// 				return
// 			}
// 		}

// 		fn(ctx)

// 		for _, middleware := range mw.postmiddleware {
// 			if next := middleware(ctx); !next {
// 				return
// 			}
// 		}
// 	})
// }

func (c *Context) View(filename string, data interface{}) error {
	body, err := os.ReadFile(filename)
	if err != nil {
		return err
	}

	t, err := template.New(filename).Parse(string(body))
	if err != nil {
		return err
	}

	return t.Execute(c.W, data)
}

func (mw *MicroWeb) Static(path string) {
	if mw.staticisset {
		return
	}

	mw.staticisset = true
	http.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.Dir(path))))
}

func (mw *MicroWeb) StaticWithPrefix(prefix, path string) {
	if mw.staticisset {
		return
	}

	if !strings.HasSuffix(prefix, "/") {
		prefix += "/"
	}

	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}

	mw.staticisset = true
	http.Handle(prefix, http.StripPrefix(prefix, http.FileServer(http.Dir(path))))
}

func (mw *MicroWeb) Get(path string, handler func(*Context)) {
	mw.addroute(path, http.MethodGet, handler)
}

func (mw *MicroWeb) Post(path string, handler func(*Context)) {
	mw.addroute(path, http.MethodPost, handler)
}

func (mw *MicroWeb) Put(path string, handler func(*Context)) {
	mw.addroute(path, http.MethodPut, handler)
}

func (mw *MicroWeb) Delete(path string, handler func(*Context)) {
	mw.addroute(path, http.MethodDelete, handler)
}

func (mw *MicroWeb) Head(path string, handler func(*Context)) {
	mw.addroute(path, http.MethodHead, handler)
}

func (mw *MicroWeb) Options(path string, handler func(*Context)) {
	mw.addroute(path, http.MethodOptions, handler)
}

func (mw *MicroWeb) Patch(path string, handler func(*Context)) {
	mw.addroute(path, http.MethodPatch, handler)
}

func (mw *MicroWeb) addroute(path, method string, handler Handler) error {
	if handler == nil {
		return errors.New("handler cannot be nil")
	}

	if method == "" {
		method = http.MethodGet
	}

	if path == "" {
		path = "/"
	}

	if path != "/" {
		path = strings.TrimSuffix(path, "/")
	}

	if mw.endpoints == nil {
		mw.endpoints = make(map[string]map[string]Handler)
	}

	_, ok := mw.endpoints[path]
	if !ok {
		mw.endpoints[path] = make(map[string]Handler)
	}

	_, ok = mw.endpoints[path][method]
	if ok {
		log.Fatal(errors.New("conflicting path " + path + ":" + method))
	}

	mw.endpoints[path][method] = handler
	return nil
}

func (mw *MicroWeb) ServeHTTP(w http.ResponseWriter, req *http.Request) {

	mw.count.Store(mw.count.Add(1))

	start := time.Now()
	defer func() {
		log.Printf("%s %s %s #%d", req.Method, req.URL.Path, time.Since(start), mw.count.Load())
	}()

	var path = req.URL.Path
	if path != "/" {
		path = strings.TrimSuffix(req.URL.Path, "/")
	}

	methods, ok := mw.endpoints[path]
	if !ok {
		http.NotFound(w, req)
		return
	}

	handler, ok := methods[req.Method]
	if !ok {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	ctx := &Context{R: req, W: w, Method: req.Method}

	for _, middleware := range mw.premiddleware {
		if next := middleware(ctx); !next {
			return
		}
	}

	handler(ctx)

	for _, middleware := range mw.postmiddleware {
		if next := middleware(ctx); !next {
			return
		}
	}
}

func (mw *MicroWeb) Listen(port int) error {
	ex := make(chan os.Signal, 2)
	signal.Notify(ex, os.Interrupt, syscall.SIGTERM)
	defer signal.Stop(ex)

	go func() {
		<-ex
		os.Exit(0)
	}()

	return http.ListenAndServe(fmt.Sprintf(":%d", port), mw)
}
