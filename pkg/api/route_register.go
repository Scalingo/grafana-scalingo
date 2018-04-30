package api

import (
	"net/http"

	macaron "gopkg.in/macaron.v1"
)

type Router interface {
	Handle(method, pattern string, handlers []macaron.Handler) *macaron.Route
	Get(pattern string, handlers ...macaron.Handler) *macaron.Route
}

// RouteRegister allows you to add routes and macaron.Handlers
// that the web server should serve.
type RouteRegister interface {
	Get(string, ...macaron.Handler)
	Post(string, ...macaron.Handler)
	Delete(string, ...macaron.Handler)
	Put(string, ...macaron.Handler)
	Patch(string, ...macaron.Handler)
	Any(string, ...macaron.Handler)

	Group(string, func(RouteRegister), ...macaron.Handler)

	Register(Router) *macaron.Router
}

type RegisterNamedMiddleware func(name string) macaron.Handler

// NewRouteRegister creates a new RouteRegister with all middlewares sent as params
func NewRouteRegister(namedMiddleware ...RegisterNamedMiddleware) RouteRegister {
	return &routeRegister{
		prefix:          "",
		routes:          []route{},
		subfixHandlers:  []macaron.Handler{},
		namedMiddleware: namedMiddleware,
	}
}

type route struct {
	method   string
	pattern  string
	handlers []macaron.Handler
}

type routeRegister struct {
	prefix          string
	subfixHandlers  []macaron.Handler
	namedMiddleware []RegisterNamedMiddleware
	routes          []route
	groups          []*routeRegister
}

func (rr *routeRegister) Group(pattern string, fn func(rr RouteRegister), handlers ...macaron.Handler) {
	group := &routeRegister{
		prefix:          rr.prefix + pattern,
		subfixHandlers:  append(rr.subfixHandlers, handlers...),
		routes:          []route{},
		namedMiddleware: rr.namedMiddleware,
	}

	fn(group)
	rr.groups = append(rr.groups, group)
}

func (rr *routeRegister) Register(router Router) *macaron.Router {
	for _, r := range rr.routes {
		// GET requests have to be added to macaron routing using Get()
		// Otherwise HEAD requests will not be allowed.
		// https://github.com/go-macaron/macaron/blob/a325110f8b392bce3e5cdeb8c44bf98078ada3be/router.go#L198
		if r.method == http.MethodGet {
			router.Get(r.pattern, r.handlers...)
		} else {
			router.Handle(r.method, r.pattern, r.handlers)
		}
	}

	for _, g := range rr.groups {
		g.Register(router)
	}

	return &macaron.Router{}
}

func (rr *routeRegister) route(pattern, method string, handlers ...macaron.Handler) {
	h := make([]macaron.Handler, 0)
	for _, fn := range rr.namedMiddleware {
		h = append(h, fn(pattern))
	}

	h = append(h, rr.subfixHandlers...)
	h = append(h, handlers...)

	rr.routes = append(rr.routes, route{
		method:   method,
		pattern:  rr.prefix + pattern,
		handlers: h,
	})
}

func (rr *routeRegister) Get(pattern string, handlers ...macaron.Handler) {
	rr.route(pattern, http.MethodGet, handlers...)
}

func (rr *routeRegister) Post(pattern string, handlers ...macaron.Handler) {
	rr.route(pattern, http.MethodPost, handlers...)
}

func (rr *routeRegister) Delete(pattern string, handlers ...macaron.Handler) {
	rr.route(pattern, http.MethodDelete, handlers...)
}

func (rr *routeRegister) Put(pattern string, handlers ...macaron.Handler) {
	rr.route(pattern, http.MethodPut, handlers...)
}

func (rr *routeRegister) Patch(pattern string, handlers ...macaron.Handler) {
	rr.route(pattern, http.MethodPatch, handlers...)
}

func (rr *routeRegister) Any(pattern string, handlers ...macaron.Handler) {
	rr.route(pattern, "*", handlers...)
}
