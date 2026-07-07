package api

import (
	"encoding/json"
	"net/http"
	"github.com/gorilla/mux"
)

func SetupREST(r *mux.Router) {
	// CORS Middleware
	r.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			w.Header().Set("Access-Control-Allow-Origin", "*")
			w.Header().Set("Access-Control-Allow-Methods", "GET, OPTIONS")
			if req.Method == "OPTIONS" {
				return
			}
			next.ServeHTTP(w, req)
		})
	})

	r.HandleFunc("/api/traces", listTraces).Methods("GET")
	r.HandleFunc("/api/traces/{traceID}", getTrace).Methods("GET")
	r.HandleFunc("/api/services", listServices).Methods("GET")
	r.HandleFunc("/api/graph", getGraph).Methods("GET")
	r.HandleFunc("/api/stats", getStats).Methods("GET")
}

func listTraces(w http.ResponseWriter, r *http.Request) {
	json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

func getTrace(w http.ResponseWriter, r *http.Request) {
	json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

func listServices(w http.ResponseWriter, r *http.Request) {
	json.NewEncoder(w).Encode([]string{})
}

func getGraph(w http.ResponseWriter, r *http.Request) {
	json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

func getStats(w http.ResponseWriter, r *http.Request) {
	json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}
