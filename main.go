package microweb

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
)

type microcontext struct {
	R      *http.Request
	W      http.ResponseWriter
	Method string
}

func (tc *microcontext) Json(v any) error {
	return json.NewEncoder(tc.W).Encode(v)
}

func (tc *microcontext) Query(key string) string {
	return tc.R.URL.Query().Get(key)
}

func (tc *microcontext) Parse(target any) error {
	body, err := io.ReadAll(tc.R.Body)
	if err != nil {
		return err
	}
	defer tc.R.Body.Close()

	return json.Unmarshal(body, target)
}

func (tc *microcontext) Body() ([]byte, error) {
	body, err := io.ReadAll(tc.R.Body)
	if err != nil {
		return nil, err
	}

	return body, nil
}

func (tc *microcontext) String(str string) error {
	tc.W.WriteHeader(http.StatusOK)
	_, err := fmt.Fprintf(tc.W, "%s", str)
	return err
}

func middle(fn func(*microcontext)) http.HandlerFunc {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := &microcontext{R: r, W: w, Method: r.Method}
		fn(ctx)
	})
}

type MicroWeb struct {
	staticisset bool
}

func New() *MicroWeb {
	return &MicroWeb{}
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

func (mw *MicroWeb) Get(path string, handler func(*microcontext)) {
	http.HandleFunc(path, middle(handler))
}

func (mw *MicroWeb) Post(path string, handler func(*microcontext)) {
	http.HandleFunc(path, middle(handler))
}

func (mw *MicroWeb) Put(path string, handler func(*microcontext)) {
	http.HandleFunc(path, middle(handler))
}

func (mw *MicroWeb) Delete(path string, handler func(*microcontext)) {
	http.HandleFunc(path, middle(handler))
}

func (mw *MicroWeb) Head(path string, handler func(*microcontext)) {
	http.HandleFunc(path, middle(handler))
}

func (mw *MicroWeb) Options(path string, handler func(*microcontext)) {
	http.HandleFunc(path, middle(handler))
}

func (mw *MicroWeb) Patch(path string, handler func(*microcontext)) {
	http.HandleFunc(path, middle(handler))
}

func (mw *MicroWeb) Listen(port int) error {
	ex := make(chan os.Signal, 2)
	signal.Notify(ex, os.Interrupt, syscall.SIGTERM)
	defer signal.Stop(ex)

	go func() {
		<-ex
		os.Exit(0)
	}()

	return http.ListenAndServe(fmt.Sprintf(":%d", port), nil)
}
