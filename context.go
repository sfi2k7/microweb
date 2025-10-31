package microweb

import (
	"context"
	"encoding/json"
	"fmt"
	"html/template"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
)

type Context struct {
	R          *http.Request
	W          http.ResponseWriter
	Method     string
	formparsed bool
	state      map[string]any
}

func (tc *Context) Json(v any) error {
	tc.W.Header().Set("Content-Type", "application/json")
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

func (tc *Context) StatusOk() {
	tc.W.WriteHeader(http.StatusOK)
}

func (tc *Context) StatusServerError() {
	tc.W.WriteHeader(http.StatusInternalServerError)
}

func (tc *Context) StatusBadRequest() {
	tc.W.WriteHeader(http.StatusBadRequest)
}

func (tc *Context) Redirect(url string, code int) {
	http.Redirect(tc.W, tc.R, url, code)
}

func (tc *Context) SetCookie(cookie *http.Cookie) {
	http.SetCookie(tc.W, cookie)
}

func (tc *Context) Cookie(name string) (*http.Cookie, error) {
	return tc.R.Cookie(name)
}

func (tc *Context) Context() context.Context {
	return tc.R.Context()
}

func (tc *Context) FormFile(name string) (multipart.File, *multipart.FileHeader, error) {
	return tc.R.FormFile(name)
}

func (tc *Context) SaveUploadedFile(file multipart.File, fileHeader *multipart.FileHeader, dst string) error {
	defer file.Close()

	// Create the destination directory if it doesn't exist
	dir := filepath.Dir(dst)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	// Create the destination file
	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()

	// Copy the uploaded file to the destination
	_, err = io.Copy(out, file)
	return err
}

func (tc *Context) MultipartForm() (*multipart.Form, error) {
	if err := tc.R.ParseMultipartForm(32 << 20); err != nil { // 32 MB max memory
		return nil, err
	}
	return tc.R.MultipartForm, nil
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
	defer tc.R.Body.Close()

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
	tc.W.Header().Set("Content-Type", "text/plain; charset=utf-8")
	tc.W.WriteHeader(http.StatusOK)
	_, err := fmt.Fprintf(tc.W, "%s", str)
	return err
}
