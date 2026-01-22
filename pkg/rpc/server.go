package rpc

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	"dogecoin.org/fractal-engine/pkg/config"
	"dogecoin.org/fractal-engine/pkg/doge"
	"dogecoin.org/fractal-engine/pkg/dogenet"
	"dogecoin.org/fractal-engine/pkg/rpc/protocol/protocolconnect"
	"dogecoin.org/fractal-engine/pkg/store"
	"golang.org/x/net/http2"
	"golang.org/x/net/http2/h2c"
	"golang.org/x/time/rate"
)

// @title			Fractal Engine API
// @version		1.0
// @description	API for managing mints and offers
type RpcServer struct {
	config     *config.Config
	quit       chan bool
	server     *http.Server
	Running    bool
	dogeClient *doge.RpcClient
}

func NewRpcServer(cfg *config.Config, store *store.TokenisationStore, gossipClient dogenet.GossipClient, dogeClient *doge.RpcClient) *RpcServer {
	mux := http.NewServeMux()

	connectService := NewConnectRpcService(store, gossipClient, cfg, dogeClient)
	connectPath, connectHandler := protocolconnect.NewFractalEngineRpcServiceHandler(connectService)
	mux.Handle(connectPath, connectHandler)

	handler := withCORS(cfg.CORSAllowedOrigins, h2c.NewHandler(mux, &http2.Server{}))

	if cfg.RpcApiKey != "" {
		handler = withSecureAPI(cfg.RpcApiKey, handler)
	}

	limiter := rate.NewLimiter(rate.Limit(cfg.RateLimitPerSecond), cfg.RateLimitPerSecond*3)
	handler = rateLimitMiddleware(limiter, handler)

	server := &http.Server{
		Addr:    cfg.RpcServerHost + ":" + cfg.RpcServerPort,
		Handler: handler,
	}

	return &RpcServer{
		config:     cfg,
		server:     server,
		quit:       make(chan bool),
		Running:    false,
		dogeClient: dogeClient,
	}
}

func rateLimitMiddleware(limiter *rate.Limiter, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !limiter.Allow() {
			http.Error(w, "Too Many Requests", http.StatusTooManyRequests)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func withSecureAPI(apiKey string, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Extract Authorization header
		authHeader := r.Header.Get("Authorization")
		if authHeader == "" || !strings.HasPrefix(authHeader, "Bearer ") {
			http.Error(w, "Forbidden", http.StatusForbidden)
			return
		}

		// Get the token part
		token := strings.TrimPrefix(authHeader, "Bearer ")

		// Validate
		if token != apiKey {
			http.Error(w, "Forbidden", http.StatusForbidden)
			return
		}

		// Proceed to actual handler
		next.ServeHTTP(w, r)
	})
}

func withCORS(allowedOrigins string, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get("Origin")
		w.Header().Set("Vary", "Origin")

		if allowedOrigins == "*" && origin != "" {
			w.Header().Set("Access-Control-Allow-Origin", "*")
		} else if origin != "" {
			for _, o := range strings.Split(allowedOrigins, ",") {
				if strings.TrimSpace(o) == origin {
					w.Header().Set("Access-Control-Allow-Origin", origin)
					break
				}
			}
		}
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")

		// Handle preflight requests
		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusNoContent)
			return
		}

		// Proceed to actual handler
		next.ServeHTTP(w, r)
	})
}

func (s *RpcServer) Start() {
	go func() {
		log.Println("Server is ready to handle requests at " + s.server.Addr)
		s.Running = true
		if err := s.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Could not listen on %s: %v\n", s.server.Addr, err)
		}
	}()

	<-s.quit
	log.Println("Shutting down server...")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := s.server.Shutdown(ctx); err != nil {
		log.Fatalf("Could not gracefully shutdown the server: %v\n", err)
	}

	log.Println("Server stopped")
}

func (s *RpcServer) Stop() {
	fmt.Println("Stopping rpc server")
	s.quit <- true
}

func respondJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}
