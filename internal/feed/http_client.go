package feed

import (
	"MrRSS/internal/utils"
	"net/http"
	"time"
)

// CreateHTTPClient creates an HTTP client with optional proxy support
// Wrapper around utils.CreateHTTPClient with default timeout for feed fetching
// DEPRECATED: Use utils.CreateHTTPClientWithUserAgent instead to avoid 403 errors
func CreateHTTPClient(proxyURL string) (*http.Client, error) {
	return utils.CreateHTTPClientWithUserAgent(
		proxyURL,
		30*time.Second,
		"Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36",
	)
}

// BuildProxyURL constructs a proxy URL from settings
// Wrapper around utils.BuildProxyURL for backward compatibility
func BuildProxyURL(proxyType, proxyHost, proxyPort, username, password string) string {
	return utils.BuildProxyURL(proxyType, proxyHost, proxyPort, username, password)
}
