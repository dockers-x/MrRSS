package summary

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"MrRSS/internal/config"
	"MrRSS/internal/utils"
)

// AISummarizer implements summarization using OpenAI-compatible APIs (GPT, Claude, etc.).
type AISummarizer struct {
	APIKey        string
	Endpoint      string
	Model         string
	SystemPrompt  string
	CustomHeaders string
	Language      string // User's language setting (e.g., "en", "zh")
	client        *http.Client
	db            DBInterface
}

// DBInterface defines the minimal database interface needed for proxy settings
type DBInterface interface {
	GetSetting(key string) (string, error)
	GetEncryptedSetting(key string) (string, error)
}

// CreateHTTPClientWithProxy creates an HTTP client with global proxy settings if enabled
func CreateHTTPClientWithProxy(db DBInterface, timeout time.Duration) (*http.Client, error) {
	var proxyURL string

	// Check if global proxy is enabled
	proxyEnabled, _ := db.GetSetting("proxy_enabled")
	if proxyEnabled == "true" {
		// Build proxy URL from global settings
		proxyType, _ := db.GetSetting("proxy_type")
		proxyHost, _ := db.GetSetting("proxy_host")
		proxyPort, _ := db.GetSetting("proxy_port")
		proxyUsername, _ := db.GetEncryptedSetting("proxy_username")
		proxyPassword, _ := db.GetEncryptedSetting("proxy_password")
		proxyURL = utils.BuildProxyURL(proxyType, proxyHost, proxyPort, proxyUsername, proxyPassword)
	}

	// Create HTTP client with or without proxy
	return utils.CreateHTTPClient(proxyURL, timeout)
}

// NewAISummarizer creates a new AI summarizer with the given credentials.
// endpoint should be the full API URL (e.g., "https://api.openai.com/v1/chat/completions" for OpenAI, "http://localhost:11434/api/generate" for Ollama)
// model should be the model name (e.g., "gpt-4o-mini", "claude-3-haiku-20240307")
// Uses global AI settings shared between translation and summarization.
// db is optional - if nil, no proxy will be used
func NewAISummarizer(apiKey, endpoint, model string) *AISummarizer {
	defaults := config.Get()
	// Use global AI endpoint and model
	if endpoint == "" {
		endpoint = defaults.AIEndpoint
	}
	if model == "" {
		model = defaults.AIModel
	}
	return &AISummarizer{
		APIKey:        apiKey,
		Endpoint:      strings.TrimSuffix(endpoint, "/"),
		Model:         model,
		SystemPrompt:  "",   // Will be set from settings when used
		CustomHeaders: "",   // Will be set from settings when used
		Language:      "en", // Default to English
		client:        &http.Client{Timeout: 30 * time.Second},
		db:            nil,
	}
}

// NewAISummarizerWithDB creates a new AI summarizer with database for proxy support
func NewAISummarizerWithDB(apiKey, endpoint, model string, db DBInterface) *AISummarizer {
	defaults := config.Get()
	if endpoint == "" {
		endpoint = defaults.AIEndpoint
	}
	if model == "" {
		model = defaults.AIModel
	}
	client, err := CreateHTTPClientWithProxy(db, 30*time.Second)
	if err != nil {
		// Fallback to default client if proxy creation fails
		client = &http.Client{Timeout: 30 * time.Second}
	}
	return &AISummarizer{
		APIKey:        apiKey,
		Endpoint:      strings.TrimSuffix(endpoint, "/"),
		Model:         model,
		SystemPrompt:  "",
		CustomHeaders: "",   // Will be set from settings when used
		Language:      "en", // Default to English
		client:        client,
		db:            db,
	}
}

// SetSystemPrompt sets a custom system prompt for the summarizer.
func (s *AISummarizer) SetSystemPrompt(prompt string) {
	s.SystemPrompt = prompt
}

// SetCustomHeaders sets custom headers for AI requests.
func (s *AISummarizer) SetCustomHeaders(headers string) {
	s.CustomHeaders = headers
}

// SetLanguage sets the language for the summarizer.
func (s *AISummarizer) SetLanguage(language string) {
	s.Language = language
}

// getDefaultSystemPrompt returns the default system prompt based on the configured language.
func (s *AISummarizer) getDefaultSystemPrompt() string {
	switch s.Language {
	case "zh":
		return "你是一个专业的文章摘要助手。请为给定的文章生成清晰、格式良好的摘要。在列出项目、特性或要点时，请优先使用项目符号或编号列表来组织内容。使摘要易于阅读和浏览。"
	default:
		return "You are a helpful AI assistant that creates clear, well-formatted summaries. When listing items, features, or points, prefer using bullet points or numbered lists to organize the content. Make the summary scannable and easy to read."
	}
}

// getUserPrompt generates a localized user prompt with target language specification.
func (s *AISummarizer) getUserPrompt(targetWords int, text string) string {
	switch s.Language {
	case "zh":
		return fmt.Sprintf("请用中文将以下内容总结为大约 %d 字：\n\n%s", targetWords, text)
	default:
		return fmt.Sprintf("Summarize the following text in English in approximately %d words:\n\n%s", targetWords, text)
	}
}

// parseCustomHeaders parses the JSON string of custom headers into a map.
func parseCustomHeaders(headersJSON string) (map[string]string, error) {
	// Return empty map if headers string is empty
	if headersJSON == "" {
		return make(map[string]string), nil
	}

	var headers map[string]string
	if err := json.Unmarshal([]byte(headersJSON), &headers); err != nil {
		return nil, fmt.Errorf("failed to parse custom headers JSON: %w", err)
	}
	return headers, nil
}

// Summarize generates a summary of the given text using an OpenAI-compatible API.
// Automatically detects and adapts to different API formats (OpenAI vs Ollama).
func (s *AISummarizer) Summarize(text string, length SummaryLength) (SummaryResult, error) {
	// Clean the text first
	cleanedText := cleanText(text)

	// Check if text is too short
	if len(cleanedText) < MinContentLength {
		return SummaryResult{
			Summary:    cleanedText,
			IsTooShort: true,
		}, nil
	}

	targetWords := getTargetWordCount(length)

	// Use custom system prompt if provided, otherwise use default
	systemPrompt := s.SystemPrompt
	if systemPrompt == "" {
		systemPrompt = s.getDefaultSystemPrompt()
	}

	// Generate localized user prompt with target language specification
	userPrompt := s.getUserPrompt(targetWords, cleanedText)

	// Try OpenAI format first
	result, thinking, err := s.tryOpenAIFormat(systemPrompt, userPrompt)
	if err == nil {
		// Count sentences in the summary
		sentences := splitSentences(result)
		return SummaryResult{
			Summary:       result,
			Thinking:      thinking,
			SentenceCount: len(sentences),
			IsTooShort:    false,
		}, nil
	}

	// If OpenAI format fails, try Ollama format
	result, thinking, err = s.tryOllamaFormat(systemPrompt, userPrompt)
	if err == nil {
		// Count sentences in the summary
		sentences := splitSentences(result)
		return SummaryResult{
			Summary:       result,
			Thinking:      thinking,
			SentenceCount: len(sentences),
			IsTooShort:    false,
		}, nil
	}

	// Both formats failed
	return SummaryResult{}, fmt.Errorf("all API formats failed: OpenAI error: %v, Ollama error: %v", err, err)
}

// tryOpenAIFormat attempts to use OpenAI-compatible API format
func (s *AISummarizer) tryOpenAIFormat(systemPrompt, userPrompt string) (string, string, error) {
	requestBody := map[string]interface{}{
		"model": s.Model,
		"messages": []map[string]string{
			{"role": "system", "content": systemPrompt},
			{"role": "user", "content": userPrompt},
		},
		"temperature": 0.3, // Low temperature for consistent summaries
	}

	jsonBody, err := json.Marshal(requestBody)
	if err != nil {
		return "", "", fmt.Errorf("failed to marshal OpenAI request: %w", err)
	}

	resp, err := s.sendRequest(jsonBody)
	if err != nil {
		return "", "", fmt.Errorf("OpenAI request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", "", fmt.Errorf("OpenAI API returned status: %d", resp.StatusCode)
	}

	var result struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", "", fmt.Errorf("failed to decode OpenAI response: %w", err)
	}

	if len(result.Choices) > 0 && result.Choices[0].Message.Content != "" {
		// Clean up the response and extract thinking
		content := strings.TrimSpace(result.Choices[0].Message.Content)
		thinking := extractThinking(content)
		summary := removeThinkingTags(content)
		return summary, thinking, nil
	}

	return "", "", fmt.Errorf("no summary found in OpenAI response")
}

// tryOllamaFormat attempts to use Ollama API format
func (s *AISummarizer) tryOllamaFormat(systemPrompt, userPrompt string) (string, string, error) {
	// Combine system and user prompts for Ollama
	fullPrompt := systemPrompt + "\n\n" + userPrompt

	requestBody := map[string]interface{}{
		"model":  s.Model,
		"prompt": fullPrompt,
		"stream": false,
	}

	jsonBody, err := json.Marshal(requestBody)
	if err != nil {
		return "", "", fmt.Errorf("failed to marshal Ollama request: %w", err)
	}

	resp, err := s.sendRequest(jsonBody)
	if err != nil {
		return "", "", fmt.Errorf("Ollama request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", "", fmt.Errorf("Ollama API returned status: %d", resp.StatusCode)
	}

	var result struct {
		Response string `json:"response"`
		Done     bool   `json:"done"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", "", fmt.Errorf("failed to decode Ollama response: %w", err)
	}

	if result.Done && result.Response != "" {
		// Clean up the response and extract thinking
		content := strings.TrimSpace(result.Response)
		thinking := extractThinking(content)
		summary := removeThinkingTags(content)
		return summary, thinking, nil
	}

	return "", "", fmt.Errorf("no summary found in Ollama response")
}

// sendRequest sends the HTTP request with proper headers
func (s *AISummarizer) sendRequest(jsonBody []byte) (*http.Response, error) {
	apiURL := s.Endpoint

	// Validate endpoint URL to prevent SSRF attacks
	parsedURL, err := url.Parse(apiURL)
	if err != nil {
		return nil, fmt.Errorf("invalid API endpoint URL: %w", err)
	}

	// Both HTTP and HTTPS are allowed
	if parsedURL.Scheme != "http" && parsedURL.Scheme != "https" {
		return nil, fmt.Errorf("API endpoint must use HTTP or HTTPS")
	}

	req, err := http.NewRequest("POST", apiURL, bytes.NewBuffer(jsonBody))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	// Only add Authorization header if API key is provided
	if s.APIKey != "" {
		req.Header.Set("Authorization", "Bearer "+s.APIKey)
	}

	// Parse and add custom headers if provided
	if s.CustomHeaders != "" {
		customHeaders, err := parseCustomHeaders(s.CustomHeaders)
		if err != nil {
			return nil, fmt.Errorf("failed to parse custom headers: %w", err)
		}
		// Apply custom headers
		for key, value := range customHeaders {
			req.Header.Set(key, value)
		}
	}

	return s.client.Do(req)
}

// isLocalEndpoint checks if a host is a local endpoint (localhost, 127.0.0.1, etc.)
// This allows using HTTP for local LLM services like Ollama
func isLocalEndpoint(host string) bool {
	// Remove port if present
	if idx := strings.LastIndex(host, ":"); idx != -1 {
		// Handle IPv6 addresses like [::1]:8080
		if !strings.Contains(host[idx:], "]") {
			host = host[:idx]
		}
	}
	// Remove brackets from IPv6 addresses
	host = strings.Trim(host, "[]")

	return host == "localhost" ||
		host == "127.0.0.1" ||
		host == "::1" ||
		strings.HasPrefix(host, "127.") ||
		host == "0.0.0.0"
}

// extractThinking extracts thinking content from <thinking> tags (case-insensitive)
func extractThinking(content string) string {
	// Try different case variations of thinking tags
	tagVariations := []struct {
		start string
		end   string
	}{
		{"<thinking>", "</thinking>"},
		{"<THINKING>", "</THINKING>"},
		{"<Thinking>", "</Thinking>"},
	}

	for _, tags := range tagVariations {
		startIndex := strings.Index(content, tags.start)
		if startIndex == -1 {
			continue
		}

		endIndex := strings.Index(content[startIndex:], tags.end)
		if endIndex == -1 {
			continue
		}

		// Extract the content between tags (excluding tags themselves)
		thinkingStart := startIndex + len(tags.start)
		thinkingEnd := startIndex + endIndex
		thinking := strings.TrimSpace(content[thinkingStart:thinkingEnd])

		return thinking
	}

	return ""
}

// removeThinkingTags removes <thinking> tags and their content from the response (case-insensitive)
func removeThinkingTags(content string) string {
	// Try different case variations of thinking tags
	tagVariations := []struct {
		start string
		end   string
	}{
		{"<thinking>", "</thinking>"},
		{"<THINKING>", "</THINKING>"},
		{"<Thinking>", "</Thinking>"},
	}

	result := content
	for _, tags := range tagVariations {
		for {
			startIndex := strings.Index(result, tags.start)
			if startIndex == -1 {
				break
			}

			endIndex := strings.Index(result[startIndex:], tags.end)
			if endIndex == -1 {
				break
			}

			// Remove the entire thinking block including tags
			thinkingEnd := startIndex + endIndex + len(tags.end)
			result = result[:startIndex] + result[thinkingEnd:]
		}
	}

	return strings.TrimSpace(result)
}
