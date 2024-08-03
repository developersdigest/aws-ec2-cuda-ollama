package main

import (
	"bytes"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"
)

const (
	ollamaURL = "http://localhost:11434"
	apiKey    = "demo"
)

func main() {
	http.HandleFunc("/v1/", handleProxy)
	fmt.Println("Server is running on 0.0.0.0:8080")
	log.Fatal(http.ListenAndServe("0.0.0.0:8080", nil))
}

func handleProxy(w http.ResponseWriter, r *http.Request) {
	// Check API key
	if !validateAPIKey(r) {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	log.Printf("Received request: %s %s", r.Method, r.URL.Path)

	// Log the request body
	logRequest(r)

	// Parse the URL of the Ollama server
	target, err := url.Parse(ollamaURL)
	if err != nil {
		http.Error(w, "Error parsing Ollama URL", http.StatusInternalServerError)
		return
	}

	// Create a reverse proxy
	proxy := httputil.NewSingleHostReverseProxy(target)

	// Modify the director to handle streaming
	originalDirector := proxy.Director
	proxy.Director = func(req *http.Request) {
		originalDirector(req)
		req.Header.Set("X-Forwarded-Host", req.Header.Get("Host"))
		req.Host = target.Host

		// Check if the client requested streaming
		if req.Header.Get("Accept") == "text/event-stream" {
			w.Header().Set("Content-Type", "text/event-stream")
			w.Header().Set("Cache-Control", "no-cache")
			w.Header().Set("Connection", "keep-alive")
		}
	}

	// Use a custom transport to handle streaming responses
	proxy.Transport = &streamTransport{http.DefaultTransport}

	proxy.ServeHTTP(w, r)
}

func validateAPIKey(r *http.Request) bool {
	// Check for API key in Authorization header
	authHeader := r.Header.Get("Authorization")
	return authHeader == "Bearer "+apiKey
}

type streamTransport struct {
	http.RoundTripper
}

func (t *streamTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	resp, err := t.RoundTripper.RoundTrip(req)
	if err != nil {
		return nil, err
	}

	if req.Header.Get("Accept") == "text/event-stream" {
		resp.Header.Set("Content-Type", "text/event-stream")
	}

	return resp, nil
}

func logRequest(r *http.Request) {
	// Read the body
	body, err := io.ReadAll(r.Body)
	if err != nil {
		log.Printf("Error reading body: %v", err)
		return
	}

	// Log the body
	log.Printf("Request Body: %s", string(body))

	// Restore the body to its original state
	r.Body = io.NopCloser(bytes.NewBuffer(body))
}
