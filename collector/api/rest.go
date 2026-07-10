package api

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/gorilla/mux"
	"go.uber.org/zap"

	"github.com/zerotrace/zerotrace/collector/graph"
	"github.com/zerotrace/zerotrace/collector/store"
)

// RESTHandler holds the dependencies needed by REST endpoints.
type RESTHandler struct {
	store *store.BadgerStore
	index *store.TraceIndex
	graph *graph.DependencyGraph
	log   *zap.Logger
}

// NewRESTHandler creates a RESTHandler.
func NewRESTHandler(s *store.BadgerStore, idx *store.TraceIndex, g *graph.DependencyGraph, log *zap.Logger) *RESTHandler {
	return &RESTHandler{store: s, index: idx, graph: g, log: log}
}

// SetupREST registers all REST endpoints on the router.
func SetupREST(r *mux.Router, h *RESTHandler) {
	// CORS middleware
	r.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			w.Header().Set("Access-Control-Allow-Origin", "*")
			w.Header().Set("Access-Control-Allow-Methods", "GET, OPTIONS")
			w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
			if req.Method == http.MethodOptions {
				w.WriteHeader(http.StatusNoContent)
				return
			}
			next.ServeHTTP(w, req)
		})
	})

	r.HandleFunc("/api/traces", h.listTraces).Methods("GET", "OPTIONS")
	r.HandleFunc("/api/traces/{traceID}", h.getTrace).Methods("GET", "OPTIONS")
	r.HandleFunc("/api/services", h.listServices).Methods("GET", "OPTIONS")
	r.HandleFunc("/api/graph", h.getGraph).Methods("GET", "OPTIONS")
	r.HandleFunc("/api/stats", h.getStats).Methods("GET", "OPTIONS")
}

// ---------------------------------------------------------------------------
// Handlers
// ---------------------------------------------------------------------------

func jsonOK(w http.ResponseWriter, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(v); err != nil {
		http.Error(w, "encode error", http.StatusInternalServerError)
	}
}

// GET /api/traces?limit=50&offset=0
// Returns a paginated list of recent trace summaries from the in-memory index.
func (h *RESTHandler) listTraces(w http.ResponseWriter, r *http.Request) {
	limit := queryInt(r, "limit", 50)
	offset := queryInt(r, "offset", 0)

	summaries := h.index.Recent(limit + offset)
	// Apply offset
	if offset < len(summaries) {
		summaries = summaries[offset:]
	} else {
		summaries = nil
	}
	if len(summaries) > limit {
		summaries = summaries[:limit]
	}
	// Return [] not null for empty results
	if summaries == nil {
		summaries = []*store.TraceSummary{}
	}
	jsonOK(w, summaries)
}

// GET /api/traces/{traceID}
// Returns all spans for a specific trace from BadgerDB.
func (h *RESTHandler) getTrace(w http.ResponseWriter, r *http.Request) {
	traceID := mux.Vars(r)["traceID"]
	if traceID == "" {
		http.Error(w, "missing traceID", http.StatusBadRequest)
		return
	}

	spans, err := h.store.GetTrace(traceID)
	if err != nil {
		h.log.Error("getTrace failed", zap.String("trace_id", traceID), zap.Error(err))
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	if len(spans) == 0 {
		http.Error(w, "trace not found", http.StatusNotFound)
		return
	}
	jsonOK(w, spans)
}

// GET /api/services
// Returns all distinct service names seen by the collector.
func (h *RESTHandler) listServices(w http.ResponseWriter, r *http.Request) {
	svcs, err := h.store.ListServices()
	if err != nil {
		h.log.Error("listServices failed", zap.Error(err))
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	if svcs == nil {
		svcs = []string{}
	}
	jsonOK(w, svcs)
}

// GET /api/graph
// Returns the live service dependency graph snapshot.
func (h *RESTHandler) getGraph(w http.ResponseWriter, r *http.Request) {
	snap := h.graph.Snapshot()
	jsonOK(w, snap)
}

// GET /api/stats
// Returns basic counters: total traces in the index, index capacity.
func (h *RESTHandler) getStats(w http.ResponseWriter, r *http.Request) {
	jsonOK(w, map[string]interface{}{
		"index_size": h.index.Len(),
		"status":     "ok",
	})
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

func queryInt(r *http.Request, key string, def int) int {
	s := r.URL.Query().Get(key)
	if s == "" {
		return def
	}
	n, err := strconv.Atoi(s)
	if err != nil || n < 0 {
		return def
	}
	return n
}
