package microweb

import "path/filepath"

type Group struct {
	r          *Router
	prefix     string
	middleware []MiddleWare
	parent     *Group
	children   []*Group
}

func (g *Group) Group(prefix string) *Group {
	return &Group{
		r:          g.r,
		parent:     g,
		prefix:     g.prefix + prefix,
		middleware: []MiddleWare{},
		children:   []*Group{},
	}
}

func (g *Group) middle(h Handler) Handler {
	return func(ctx *Context) {
		for _, m := range g.middleware {
			if !m(ctx) {
				return
			}
		}

		h(ctx)
	}
}

func (g *Group) Use(middlewares ...MiddleWare) {
	g.middleware = append(g.middleware, middlewares...)
}

func (g *Group) Get(path string, handler Handler) {
	g.r.Get(filepath.Join(g.prefix, path), g.middle(handler))
}

func (g *Group) Post(path string, handler Handler) {
	g.r.Post(filepath.Join(g.prefix, path), g.middle(handler))
}

func (g *Group) Put(path string, handler Handler) {
	g.r.Put(filepath.Join(g.prefix, path), g.middle(handler))
}

func (g *Group) Delete(path string, handler Handler) {
	g.r.Delete(filepath.Join(g.prefix, path), g.middle(handler))
}

func (g *Group) Patch(path string, handler Handler) {
	g.r.Patch(filepath.Join(g.prefix, path), g.middle(handler))
}

func (g *Group) Options(path string, handler Handler) {
	g.r.Options(filepath.Join(g.prefix, path), g.middle(handler))
}

func (g *Group) Head(path string, handler Handler) {
	g.r.Head(filepath.Join(g.prefix, path), g.middle(handler))
}
