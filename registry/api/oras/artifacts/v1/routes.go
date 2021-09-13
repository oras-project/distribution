package v1

import "github.com/gorilla/mux"

// The following are definitions of the name under which all V1 routes are
// registered. These symbols can be used to look up a route based on the name.
const (
	RouteNameReferrers = "referrers"
)

// RouterWithPrefix builds a gorilla router with a configured prefix
// on all routes.
func AddPaths(router *mux.Router) *mux.Router {
	if router == nil {
		return nil
	}

	for _, descriptor := range routeDescriptors {
		router.Path(descriptor.Path).Name(descriptor.Name)
	}

	return router
}
