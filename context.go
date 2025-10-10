package microweb

import (
	"encoding/json"
	"fmt"
	"html/template"
	"io"
	"net/http"
	"os"
)

type Context struct {
	R          *http.Request
	W          http.ResponseWriter
	Method     string
	formparsed bool
	state      map[string]any
}

func (tc *Context) Json(v any) error {
	return json.NewEncoder(tc.W).Encode(v)
}

func (tc *Context) Query(key string) string {
	return tc.R.URL.Query().Get(key)
}

func (tc *Context) Status(status int) {
	tc.W.WriteHeader(status)
}

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

func (c *Context) Param(key string) string {
	return c.R.PathValue(key)
}

func (c *Context) Header(key string) string {
	return c.R.Header.Get(key)
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

func (tc *Context) Set(k string, v any) {
	tc.state[k] = v
}

func (tc *Context) Get(k string) any {
	if v, ok := tc.state[k]; ok {
		return v
	}

	return nil
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

func (tc *Context) FormValue(key string) string {
	if !tc.formparsed {
		tc.R.ParseForm()
		tc.formparsed = true
	}

	return tc.R.FormValue(key)
}

func (tc *Context) String(str string) error {
	tc.W.WriteHeader(http.StatusOK)
	_, err := fmt.Fprintf(tc.W, "%s", str)
	return err
}
