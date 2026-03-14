package dashboard

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/coder/websocket"
	"github.com/coder/websocket/wsjson"
	"github.com/go-chi/chi/v5"
	chiMiddleware "github.com/go-chi/chi/v5/middleware"
	"github.com/user/telegram-claude-bot/internal/events"
	"github.com/user/telegram-claude-bot/internal/store"
)

// StartServer starts the dashboard HTTP + WebSocket server.
func StartServer(ctx context.Context, addr string, cfg *store.GlobalConfig) error {
	hub := newWSHub()
	go hub.run(ctx)

	// Subscribe to all events and broadcast to WebSocket clients
	events.Bus.On("*", func(e events.EventData) {
		hub.broadcast(e)
	})

	r := chi.NewRouter()
	r.Use(chiMiddleware.Recoverer)
	r.Use(chiMiddleware.RealIP)

	// Health endpoint
	r.Get("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"status":    "ok",
			"uptime":    time.Since(startTime).String(),
			"timestamp": time.Now().UTC().Format(time.RFC3339),
		})
	})

	// WebSocket endpoint
	r.Get("/ws", func(w http.ResponseWriter, r *http.Request) {
		hub.handleWS(w, r)
	})

	// Admin API (with API key auth)
	r.Route("/api", func(r chi.Router) {
		r.Use(apiKeyAuth(cfg.AdminAPIKey))

		r.Get("/users", handleListUsers)
		r.Get("/stats", handleStats)
		r.Get("/config", handleGetConfig)
		r.Post("/config", handleSetConfig)
		r.Get("/logs", handleGetLogs)
	})

	// Static files (dashboard)
	r.Get("/", func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, "static/dashboard.html")
	})
	r.Handle("/static/*", http.StripPrefix("/static/", http.FileServer(http.Dir("static"))))

	srv := &http.Server{Addr: addr, Handler: r}

	go func() {
		<-ctx.Done()
		shutCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		srv.Shutdown(shutCtx)
	}()

	return srv.ListenAndServe()
}

var startTime = time.Now()

// --- API Key Auth Middleware ---

func apiKeyAuth(apiKey string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if apiKey == "" {
				next.ServeHTTP(w, r)
				return
			}
			key := r.Header.Get("X-API-Key")
			if key == "" {
				key = r.URL.Query().Get("api_key")
			}
			if key != apiKey {
				http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// --- API Handlers ---

func handleListUsers(w http.ResponseWriter, r *http.Request) {
	users, err := store.ListAllUsers()
	if err != nil {
		http.Error(w, fmt.Sprintf(`{"error":"%s"}`, err), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(users)
}

func handleStats(w http.ResponseWriter, r *http.Request) {
	stats, err := store.GetStats()
	if err != nil {
		http.Error(w, fmt.Sprintf(`{"error":"%s"}`, err), http.StatusInternalServerError)
		return
	}
	costStats := store.GetAllCostStats()
	result := map[string]any{
		"logs":  stats,
		"costs": costStats,
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(result)
}

func handleGetConfig(w http.ResponseWriter, r *http.Request) {
	cfg, err := store.GetAllConfig()
	if err != nil {
		http.Error(w, fmt.Sprintf(`{"error":"%s"}`, err), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(cfg)
}

func handleSetConfig(w http.ResponseWriter, r *http.Request) {
	var body map[string]string
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, `{"error":"invalid json"}`, http.StatusBadRequest)
		return
	}
	for k, v := range body {
		if err := store.SetConfig(k, v); err != nil {
			http.Error(w, fmt.Sprintf(`{"error":"%s"}`, err), http.StatusInternalServerError)
			return
		}
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

func handleGetLogs(w http.ResponseWriter, r *http.Request) {
	date := r.URL.Query().Get("date")
	if date == "" {
		date = time.Now().Format("2006-01-02")
	}
	logs, err := store.GetLogs(date, 100)
	if err != nil {
		http.Error(w, fmt.Sprintf(`{"error":"%s"}`, err), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(logs)
}

// --- WebSocket Hub ---

type wsClient struct {
	conn *websocket.Conn
	ctx  context.Context
}

type wsHub struct {
	mu      sync.RWMutex
	clients map[*wsClient]bool
	msgCh   chan events.EventData
}

func newWSHub() *wsHub {
	return &wsHub{
		clients: make(map[*wsClient]bool),
		msgCh:   make(chan events.EventData, 256),
	}
}

func (h *wsHub) run(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case msg := <-h.msgCh:
			h.mu.RLock()
			for client := range h.clients {
				go func(c *wsClient) {
					writeCtx, cancel := context.WithTimeout(c.ctx, 5*time.Second)
					defer cancel()
					if err := wsjson.Write(writeCtx, c.conn, msg); err != nil {
						h.removeClient(c)
					}
				}(client)
			}
			h.mu.RUnlock()
		}
	}
}

func (h *wsHub) broadcast(event events.EventData) {
	select {
	case h.msgCh <- event:
	default:
		// Drop if buffer full
	}
}

func (h *wsHub) addClient(c *wsClient) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.clients[c] = true
}

func (h *wsHub) removeClient(c *wsClient) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if _, ok := h.clients[c]; ok {
		delete(h.clients, c)
		c.conn.Close(websocket.StatusNormalClosure, "")
	}
}

func (h *wsHub) handleWS(w http.ResponseWriter, r *http.Request) {
	conn, err := websocket.Accept(w, r, &websocket.AcceptOptions{
		InsecureSkipVerify: true,
	})
	if err != nil {
		log.Printf("WebSocket accept error: %v", err)
		return
	}

	client := &wsClient{conn: conn, ctx: r.Context()}
	h.addClient(client)
	defer h.removeClient(client)

	// Keep connection alive by reading (discard incoming messages)
	for {
		_, _, err := conn.Read(r.Context())
		if err != nil {
			return
		}
	}
}
