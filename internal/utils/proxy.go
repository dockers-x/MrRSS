package utils

import (
	"crypto/tls"
	"fmt"
	"net/http"
	"net/url"
	"time"
)

// BuildProxyURL constructs a proxy URL from settings
func BuildProxyURL(proxyType, proxyHost, proxyPort, username, password string) string {
	if proxyHost == "" || proxyPort == "" {
		return ""
	}

	// Build auth string if username is provided
	auth := ""
	if username != "" {
		if password != "" {
			auth = username + ":" + password + "@"
		} else {
			auth = username + "@"
		}
	}

	return fmt.Sprintf("%s://%s%s:%s", proxyType, auth, proxyHost, proxyPort)
}

// CreateHTTPClient creates an HTTP client with optional proxy support
// This is the canonical implementation with proper TLS config and connection pooling
func CreateHTTPClient(proxyURL string, timeout time.Duration) (*http.Client, error) {
	transport := &http.Transport{
		TLSClientConfig: &tls.Config{
			MinVersion: tls.VersionTLS12,
		},
		MaxIdleConns:        50, // Reduced from 100 to prevent connection exhaustion
		MaxIdleConnsPerHost: 5,  // Reduced from 10 to limit connections per host
		IdleConnTimeout:     90 * time.Second,
		// Disable HTTP/2 for RSS feeds - it can cause performance issues
		// HTTP/1.1 is more reliable and faster for simple RSS feed fetching
		ForceAttemptHTTP2: false,
		// Write buffer size
		WriteBufferSize: 32 * 1024, // 32KB
		// Read buffer size
		ReadBufferSize: 32 * 1024, // 32KB
	}

	// Configure proxy if provided
	if proxyURL != "" {
		parsedProxy, err := url.Parse(proxyURL)
		if err != nil {
			return nil, fmt.Errorf("invalid proxy URL: %w", err)
		}
		transport.Proxy = http.ProxyURL(parsedProxy)
	}

	client := &http.Client{
		Transport: transport,
		Timeout:   timeout,
	}

	return client, nil
}

// RoundTripFunc is an adapter to allow the use of ordinary functions as http.RoundTripper
type RoundTripFunc func(req *http.Request) (*http.Response, error)

// RoundTrip implements http.RoundTripper
func (rt RoundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return rt(req)
}

// CreateHTTPClientWithUserAgent creates an HTTP client with a custom User-Agent
// This is important because some RSS servers block requests without a proper User-Agent
func CreateHTTPClientWithUserAgent(proxyURL string, timeout time.Duration, userAgent string) (*http.Client, error) {
	baseClient, err := CreateHTTPClient(proxyURL, timeout)
	if err != nil {
		return nil, err
	}

	// Wrap the transport to add User-Agent to all requests
	originalTransport := baseClient.Transport
	baseClient.Transport = RoundTripFunc(func(req *http.Request) (*http.Response, error) {
		// Set User-Agent if not already set
		if req.Header.Get("User-Agent") == "" {
			req.Header.Set("User-Agent", userAgent)
		}
		// Set Accept header for RSS feeds
		if req.Header.Get("Accept") == "" {
			req.Header.Set("Accept", "application/rss+xml, application/xml, text/xml, */*")
		}
		return originalTransport.RoundTrip(req)
	})

	return baseClient, nil
}
