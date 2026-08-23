package helpers

import (
	"errors"
	"log"
	"net"
	"net/http"
	"strings"
)

func GetClientIP(r *http.Request) string {
	// Prefer X-Real-IP: set by trusted edge nginx to $remote_addr.
	if realIP := r.Header.Get("X-Real-IP"); realIP != "" {
		return realIP
	}

	// X-Forwarded-For: nginx overwrites this with $remote_addr at the edge.
	if fwd := r.Header.Get("X-Forwarded-For"); fwd != "" {
		ips := strings.Split(fwd, ",")
		return strings.TrimSpace(ips[0])
	}

	// Fallback when not behind a proxy (direct :8080 access)
	ip, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return ip
}

func GetRequestApiKey(r *http.Request) (string, error) {
	apiKey := r.Header.Get("X-API-KEY")
	if apiKey != "" {
		apiKey = apiKey
	} else {
		log.Printf("Failed to retrieve a valid api-key")
		return "", errors.New("Failed to retrieve a valid api-key")
	}
	return apiKey, nil
}
