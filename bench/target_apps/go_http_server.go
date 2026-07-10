package main

import (
	"fmt"
	"io"
	"log"
	"net/http"
	"time"
)

func main() {
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		// Call Flask — this creates the cross-service TCP flow the correlator needs
		client := &http.Client{Timeout: 5 * time.Second}
		resp, err := client.Get("http://127.0.0.1:5001/api/health")
		if err != nil {
			http.Error(w, "upstream error: "+err.Error(), 502)
			return
		}
		defer resp.Body.Close()
		body, _ := io.ReadAll(resp.Body)
		fmt.Fprintf(w, `{"status":"ok","upstream":%d,"body":"%s"}`, resp.StatusCode, string(body))
	})

	http.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, `{"status":"ok"}`)
	})

	log.Println("go_http_server listening on :8081")
	log.Fatal(http.ListenAndServe(":8081", nil))
}
