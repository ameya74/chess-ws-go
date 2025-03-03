package middleware

import (
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"
)

type responseWriter struct {
	http.ResponseWriter
	status      int
	wroteHeader bool
	size        int64
}

func wrapResponseWriter(w http.ResponseWriter) *responseWriter {
	return &responseWriter{ResponseWriter: w}
}

func (rw *responseWriter) WriteHeader(code int) {
	if !rw.wroteHeader {
		rw.status = code
		rw.wroteHeader = true
		rw.ResponseWriter.WriteHeader(code)
	}
}

func (rw *responseWriter) Write(buf []byte) (int, error) {
	if !rw.wroteHeader {
		rw.WriteHeader(http.StatusOK)
	}
	n, err := rw.ResponseWriter.Write(buf)
	rw.size += int64(n)
	return n, err
}

// LoggingMiddleware wraps an http.Handler and logs request information
func LoggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		wrapped := wrapResponseWriter(w)

		// Get real IP considering forwarded headers
		realIP := r.Header.Get("X-Real-IP")
		if realIP == "" {
			realIP = r.Header.Get("X-Forwarded-For")
			if realIP == "" {
				realIP = r.RemoteAddr
			}
		}

		// Process request
		next.ServeHTTP(wrapped, r)

		// Calculate duration
		duration := time.Since(start)

		// Get user agent and clean it up
		userAgent := r.UserAgent()
		if len(userAgent) > 100 {
			userAgent = userAgent[:100] + "..."
		}

		// Log entry
		logEntry := fmt.Sprintf(
			"[%s] %s - %s \"%s %s %s\" %d %d \"%s\" \"%s\" (took: %v)",
			time.Now().Format("2006/01/02 15:04:05"),
			realIP,
			r.Host,
			r.Method,
			r.URL.Path,
			r.Proto,
			wrapped.status,
			wrapped.size,
			strings.Replace(r.Referer(), "\"", "'", -1),
			userAgent,
			duration,
		)

		log.Println(logEntry)
	})
}
