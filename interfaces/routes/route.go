package routes

import (
	"hash-signing-service/config"
	"hash-signing-service/interfaces/handlers"
	"hash-signing-service/interfaces/middleware"

	"github.com/gorilla/mux"
)

type Route struct {
	config *config.Config
}

func New(conf *config.Config) *Route {
	return &Route{
		config: conf,
	}
}

// Init is a method to initialize gin engine.
func (r *Route) Init() *mux.Router {
	router := mux.NewRouter()

	// middleware to all request
	router.Use(middleware.Logger(r.config))
	// set config to the context
	router.Use(middleware.SetConfigInContext(r.config))

	router.HandleFunc("/", handlers.RootHandler).Methods("GET")
	router.HandleFunc("/hash_signing", handlers.Signing).Methods("POST")

	return router
}
