package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"time"
)

var (
	userSvcURL string
	noteSvcURL string
	client     = &http.Client{Timeout: 5 * time.Second}
)

func init() {
	userSvcURL = getEnv("USER_SVC_URL", "http://localhost:8081")
	noteSvcURL = getEnv("NOTE_SVC_URL", "http://localhost:8082")
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func main() {
	http.HandleFunc("/health", healthHandler)
	http.HandleFunc("/api/users", proxyHandler(userSvcURL, "/users"))
	http.HandleFunc("/api/users/", proxyHandler(userSvcURL, "/users/"))
	http.HandleFunc("/api/notes", proxyHandler(noteSvcURL, "/notes"))
	http.HandleFunc("/api/notes/", proxyHandler(noteSvcURL, "/notes/"))

	port := getEnv("PORT", "8080")
	log.Printf("api-gateway starting on :%s", port)
	log.Printf("user-svc: %s, note-svc: %s", userSvcURL, noteSvcURL)
	log.Fatal(http.ListenAndServe(":"+port, nil))
}

func healthHandler(w http.ResponseWriter, r *http.Request) {
	json.NewEncoder(w).Encode(map[string]string{
		"status":  "ok",
		"service": "api-gateway",
	})
}

func proxyHandler(targetBase, pathPrefix string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Build target URL
		targetURL := targetBase + r.URL.Path[len("/api"):]
		if r.URL.RawQuery != "" {
			targetURL += "?" + r.URL.RawQuery
		}

		log.Printf("[proxy] %s %s -> %s", r.Method, r.URL.Path, targetURL)

		// Create proxy request
		proxyReq, err := http.NewRequest(r.Method, targetURL, r.Body)
		if err != nil {
			http.Error(w, fmt.Sprintf("proxy error: %v", err), http.StatusBadGateway)
			return
		}
		proxyReq.Header = r.Header.Clone()

		// Forward request
		resp, err := client.Do(proxyReq)
		if err != nil {
			http.Error(w, fmt.Sprintf("upstream error: %v", err), http.StatusBadGateway)
			return
		}
		defer resp.Body.Close()

		// Copy response
		for k, vv := range resp.Header {
			for _, v := range vv {
				w.Header().Add(k, v)
			}
		}
		w.WriteHeader(resp.StatusCode)
		io.Copy(w, resp.Body)
	}
}
