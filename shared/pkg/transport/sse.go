package transport

import (
	"fmt"
	"net/http"
	"time"
)

// SSEHandler provides basic SSE connection handling
// Services can extend this to add custom message handling
type SSEHandler struct{}

// NewSSEHandler creates a new SSE handler
func NewSSEHandler() *SSEHandler {
	return &SSEHandler{}
}

// HandleConnection handles an SSE GET request (establishes long-lived connection)
func (h *SSEHandler) HandleConnection(w http.ResponseWriter, r *http.Request) {
	// Set SSE headers
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")

	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "Streaming unsupported", http.StatusInternalServerError)
		return
	}

	// Send connection established event
	fmt.Fprintf(w, "event: endpoint\ndata: /sse\n\n")
	flusher.Flush()

	// Keep connection alive with periodic pings
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-r.Context().Done():
			return
		case <-ticker.C:
			fmt.Fprintf(w, ": ping\n\n")
			flusher.Flush()
		}
	}
}

// SetSSEHeaders sets the required headers for SSE responses
func SetSSEHeaders(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")
}

// WriteSSEMessage writes an SSE message event
func WriteSSEMessage(w http.ResponseWriter, data string) error {
	_, err := fmt.Fprintf(w, "event: message\ndata: %s\n\n", data)
	if flusher, ok := w.(http.Flusher); ok {
		flusher.Flush()
	}
	return err
}

// WriteSSEError writes an SSE error event
func WriteSSEError(w http.ResponseWriter, errorMsg string) error {
	_, err := fmt.Fprintf(w, "event: error\ndata: %s\n\n", errorMsg)
	if flusher, ok := w.(http.Flusher); ok {
		flusher.Flush()
	}
	return err
}
