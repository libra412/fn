package transport

import "github.com/libra412/fn/plugins/endpoint"

type router struct {
	routeMap map[string]endpoint.Endpoint
}

var route *router

// singleton
func newRouter() *router {
	if route == nil {
		route = &router{
			routeMap: make(map[string]endpoint.Endpoint),
		}
	}
	return route
}

// InitRouter initializes the router with the provided list of register functions.
func InitRouter(list ...register) {
	for _, reg := range list {
		reg.RegisterEndpoint(registerHandler)
	}
}

type register interface {
	RegisterEndpoint(func(string, endpoint.Endpoint))
}

// 注册路由
func registerHandler(method string, endpoint endpoint.Endpoint) {
	newRouter().registerEndpoint(method, endpoint)
}

func GetRouter() *router {
	if route == nil {
		route = newRouter()
	}
	return route
}

func (r *router) GetEndpoint(method string) endpoint.Endpoint {
	return r.routeMap[method]
}

func (r *router) registerEndpoint(method string, endpoint endpoint.Endpoint) {
	r.routeMap[method] = endpoint
}
