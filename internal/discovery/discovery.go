// Package discovery provides blog discovery functionality
package discovery

import (
	"context"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/PuerkitoBio/goquery"
	"github.com/mmcdole/gofeed"
)

// ProgressCallback is called with progress updates during discovery
type ProgressCallback func(progress Progress)

// Progress represents the current discovery progress
type Progress struct {
	Stage       string `json:"stage"`        // Current stage (e.g., "fetching_homepage", "finding_friend_links", "checking_rss")
	Message     string `json:"message"`      // Human-readable message
	Detail      string `json:"detail"`       // Additional detail (e.g., current URL being checked)
	Current     int    `json:"current"`      // Current item index
	Total       int    `json:"total"`        // Total items to process
	FeedName    string `json:"feed_name"`    // Name of the feed being processed (for batch discovery)
	FoundCount  int    `json:"found_count"`  // Number of feeds found so far
}

// DiscoveredBlog represents a blog found through friend links
type DiscoveredBlog struct {
	Name           string          `json:"name"`
	Homepage       string          `json:"homepage"`
	RSSFeed        string          `json:"rss_feed"`
	IconURL        string          `json:"icon_url"`
	RecentArticles []RecentArticle `json:"recent_articles"`
}

// RecentArticle represents a recent article with title and date
type RecentArticle struct {
	Title string `json:"title"`
	Date  string `json:"date"` // ISO 8601 format or relative time
}

// Service handles blog discovery operations
type Service struct {
	client     *http.Client
	feedParser *gofeed.Parser
}

// NewService creates a new discovery service
func NewService() *Service {
	return &Service{
		client: &http.Client{
			Timeout: 15 * time.Second,
			CheckRedirect: func(req *http.Request, via []*http.Request) error {
				if len(via) >= 5 {
					return fmt.Errorf("too many redirects")
				}
				return nil
			},
		},
		feedParser: gofeed.NewParser(),
	}
}

// DiscoverFromFeed discovers blogs from a feed's homepage
func (s *Service) DiscoverFromFeed(ctx context.Context, feedURL string) ([]DiscoveredBlog, error) {
	return s.DiscoverFromFeedWithProgress(ctx, feedURL, nil)
}

// DiscoverFromFeedWithProgress discovers blogs from a feed's homepage with progress updates
func (s *Service) DiscoverFromFeedWithProgress(ctx context.Context, feedURL string, progressCb ProgressCallback) ([]DiscoveredBlog, error) {
	// Report progress: fetching homepage
	if progressCb != nil {
		progressCb(Progress{
			Stage:   "fetching_homepage",
			Message: "Fetching homepage from feed",
			Detail:  feedURL,
		})
	}

	// First, try to parse the feed to get the homepage link
	homepage, err := s.getFeedHomepage(ctx, feedURL)
	if err != nil {
		return nil, fmt.Errorf("failed to get homepage from feed: %w", err)
	}

	// Report progress: finding friend links
	if progressCb != nil {
		progressCb(Progress{
			Stage:   "finding_friend_links",
			Message: "Searching for friend links",
			Detail:  homepage,
		})
	}

	// Fetch the homepage HTML
	friendLinks, err := s.findFriendLinksWithProgress(ctx, homepage, progressCb)
	if err != nil {
		return nil, fmt.Errorf("failed to find friend links: %w", err)
	}

	if len(friendLinks) == 0 {
		return []DiscoveredBlog{}, nil
	}

	// Report progress: checking RSS feeds
	if progressCb != nil {
		progressCb(Progress{
			Stage:   "checking_rss",
			Message: "Checking RSS feeds",
			Total:   len(friendLinks),
		})
	}

	// Discover RSS feeds from friend links (concurrent)
	discovered := s.discoverRSSFeedsWithProgress(ctx, friendLinks, progressCb)

	return discovered, nil
}

// getFeedHomepage extracts the homepage URL from a feed
func (s *Service) getFeedHomepage(ctx context.Context, feedURL string) (string, error) {
	feed, err := s.feedParser.ParseURLWithContext(feedURL, ctx)
	if err != nil {
		return "", err
	}

	if feed.Link != "" {
		return feed.Link, nil
	}

	// Fallback: try to extract base URL from feed URL
	u, err := url.Parse(feedURL)
	if err != nil {
		return "", err
	}

	return fmt.Sprintf("%s://%s", u.Scheme, u.Host), nil
}

// findFriendLinks searches for friend link pages and extracts links
func (s *Service) findFriendLinks(ctx context.Context, homepage string) ([]string, error) {
	return s.findFriendLinksWithProgress(ctx, homepage, nil)
}

// findFriendLinksWithProgress searches for friend link pages with progress updates
func (s *Service) findFriendLinksWithProgress(ctx context.Context, homepage string, progressCb ProgressCallback) ([]string, error) {
	// Try to find friend link page
	friendPageURL, err := s.findFriendLinkPage(ctx, homepage)
	if err != nil {
		log.Printf("Could not find friend link page, trying homepage: %v", err)
		friendPageURL = homepage
	}

	if progressCb != nil {
		progressCb(Progress{
			Stage:   "fetching_friend_page",
			Message: "Fetching friend links page",
			Detail:  friendPageURL,
		})
	}

	// Fetch and parse the friend link page
	doc, err := s.fetchHTML(ctx, friendPageURL)
	if err != nil {
		return nil, err
	}

	// Extract all external links
	links := s.extractExternalLinks(doc, friendPageURL)

	if progressCb != nil {
		progressCb(Progress{
			Stage:   "found_links",
			Message: fmt.Sprintf("Found %d potential blog links", len(links)),
			Total:   len(links),
		})
	}

	return links, nil
}

// findFriendLinkPage searches for a friend link page
func (s *Service) findFriendLinkPage(ctx context.Context, homepage string) (string, error) {
	doc, err := s.fetchHTML(ctx, homepage)
	if err != nil {
		return "", err
	}

	// Expanded patterns for friend link pages (multiple languages and variations)
	patterns := []string{
		// Chinese patterns
		"友链", "友情链接", "博客友链", "友情", "朋友们", "小伙伴", "友邻", "链接",
		// English patterns
		"blogroll", "friends", "links", "friend links", "blog links",
		"link", "buddy", "buddies", "partner", "partners", "bloggers",
		"recommended", "blog roll", "favorite blogs", "other blogs",
		// Common URL paths
		"about/links", "friends.html", "links.html", "blogroll.html",
		"friend", "flink", "link-exchange",
	}

	var foundURL string
	doc.Find("a").Each(func(i int, sel *goquery.Selection) {
		if foundURL != "" {
			return
		}

		href, exists := sel.Attr("href")
		if !exists {
			return
		}

		text := strings.ToLower(strings.TrimSpace(sel.Text()))
		hrefLower := strings.ToLower(href)

		// Check if link text or href contains friend link patterns
		for _, pattern := range patterns {
			if strings.Contains(text, pattern) || strings.Contains(hrefLower, pattern) {
				// Resolve relative URLs
				if absURL := s.resolveURL(homepage, href); absURL != "" {
					foundURL = absURL
					return
				}
			}
		}
	})

	if foundURL != "" {
		return foundURL, nil
	}

	return "", fmt.Errorf("friend link page not found")
}

// fetchHTML fetches and parses HTML from a URL
func (s *Service) fetchHTML(ctx context.Context, urlStr string) (*goquery.Document, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", urlStr, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("User-Agent", "MrRSS/1.0 (Blog Discovery Bot)")

	resp, err := s.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP error: %d", resp.StatusCode)
	}

	doc, err := goquery.NewDocumentFromReader(resp.Body)
	if err != nil {
		return nil, err
	}

	return doc, nil
}

// extractExternalLinks extracts all external links from a page
func (s *Service) extractExternalLinks(doc *goquery.Document, baseURL string) []string {
	seen := make(map[string]bool)
	var links []string

	baseU, err := url.Parse(baseURL)
	if err != nil {
		return links
	}

	doc.Find("a[href]").Each(func(i int, sel *goquery.Selection) {
		href, _ := sel.Attr("href")
		absURL := s.resolveURL(baseURL, href)
		if absURL == "" {
			return
		}

		u, err := url.Parse(absURL)
		if err != nil {
			return
		}

		// Only include external links (different domain)
		if u.Host != baseU.Host && u.Host != "" {
			// Skip common non-blog domains
			if s.isValidBlogDomain(u.Host) && !seen[absURL] {
				seen[absURL] = true
				links = append(links, absURL)
			}
		}
	})

	return links
}

// isValidBlogDomain checks if a domain is likely a blog
func (s *Service) isValidBlogDomain(host string) bool {
	// Skip common non-blog domains
	skipDomains := []string{
		"facebook.com", "twitter.com", "instagram.com", "linkedin.com",
		"youtube.com", "github.com", "stackoverflow.com", "reddit.com",
		"weibo.com", "zhihu.com", "bilibili.com", "douban.com",
		"google.com", "baidu.com", "bing.com", "yahoo.com",
	}

	hostLower := strings.ToLower(host)
	for _, skip := range skipDomains {
		if strings.Contains(hostLower, skip) {
			return false
		}
	}

	return true
}

// resolveURL resolves a relative URL to an absolute URL
func (s *Service) resolveURL(base, href string) string {
	if href == "" {
		return ""
	}

	baseURL, err := url.Parse(base)
	if err != nil {
		return ""
	}

	hrefURL, err := url.Parse(href)
	if err != nil {
		return ""
	}

	return baseURL.ResolveReference(hrefURL).String()
}

// discoverRSSFeeds discovers RSS feeds from a list of blog URLs
func (s *Service) discoverRSSFeeds(ctx context.Context, blogURLs []string) []DiscoveredBlog {
	return s.discoverRSSFeedsWithProgress(ctx, blogURLs, nil)
}

// discoverRSSFeedsWithProgress discovers RSS feeds with progress updates
func (s *Service) discoverRSSFeedsWithProgress(ctx context.Context, blogURLs []string, progressCb ProgressCallback) []DiscoveredBlog {
	var wg sync.WaitGroup
	results := make(chan DiscoveredBlog, len(blogURLs))
	sem := make(chan struct{}, 15) // Increased concurrency from 10 to 15

	// Track progress
	var progressMu sync.Mutex
	processed := 0
	foundCount := 0
	total := len(blogURLs)

	for _, blogURL := range blogURLs {
		select {
		case <-ctx.Done():
			break
		default:
		}

		wg.Add(1)
		sem <- struct{}{}

		go func(u string) {
			defer wg.Done()
			defer func() { <-sem }()

			// Report progress
			if progressCb != nil {
				progressMu.Lock()
				processed++
				currentProcessed := processed
				currentFound := foundCount
				progressMu.Unlock()

				progressCb(Progress{
					Stage:      "checking_rss",
					Message:    "Checking RSS feed",
					Detail:     u,
					Current:    currentProcessed,
					Total:      total,
					FoundCount: currentFound,
				})
			}

			if blog, err := s.discoverBlogRSS(ctx, u); err == nil {
				progressMu.Lock()
				foundCount++
				progressMu.Unlock()
				results <- blog
			}
		}(blogURL)
	}

	go func() {
		wg.Wait()
		close(results)
	}()

	var discovered []DiscoveredBlog
	for blog := range results {
		discovered = append(discovered, blog)
	}

	return discovered
}

// discoverBlogRSS discovers RSS feed for a single blog
func (s *Service) discoverBlogRSS(ctx context.Context, blogURL string) (DiscoveredBlog, error) {
	// Try to find RSS feed URL
	rssURL, err := s.findRSSFeed(ctx, blogURL)
	if err != nil {
		return DiscoveredBlog{}, err
	}

	// Parse the RSS feed to get blog info
	feed, err := s.feedParser.ParseURLWithContext(rssURL, ctx)
	if err != nil {
		return DiscoveredBlog{}, err
	}

	// Extract recent articles (max 3)
	var recentArticles []RecentArticle
	for i := 0; i < len(feed.Items) && i < 3; i++ {
		item := feed.Items[i]
		dateStr := ""
		if item.PublishedParsed != nil {
			// Format as relative time or date
			dateStr = item.PublishedParsed.Format("2006-01-02")
		}
		recentArticles = append(recentArticles, RecentArticle{
			Title: item.Title,
			Date:  dateStr,
		})
	}

	// Get favicon
	iconURL := s.getFavicon(blogURL)

	return DiscoveredBlog{
		Name:           feed.Title,
		Homepage:       blogURL,
		RSSFeed:        rssURL,
		IconURL:        iconURL,
		RecentArticles: recentArticles,
	}, nil
}

// findRSSFeed finds the RSS feed URL for a blog
func (s *Service) findRSSFeed(ctx context.Context, blogURL string) (string, error) {
	// Common RSS feed paths to try
	u, err := url.Parse(blogURL)
	if err != nil {
		return "", err
	}

	baseURL := fmt.Sprintf("%s://%s", u.Scheme, u.Host)

	// First, try to parse HTML and find RSS link in <head> - this is usually the most reliable
	doc, err := s.fetchHTML(ctx, blogURL)
	if err == nil {
		var foundFeed string
		doc.Find("link[type='application/rss+xml'], link[type='application/atom+xml'], link[rel='alternate'][type*='xml']").Each(func(i int, sel *goquery.Selection) {
			if foundFeed != "" {
				return
			}
			if href, exists := sel.Attr("href"); exists {
				foundFeed = s.resolveURL(blogURL, href)
			}
		})

		if foundFeed != "" && s.isValidFeed(ctx, foundFeed) {
			return foundFeed, nil
		}
	}

	// Expanded common RSS/Atom feed paths
	commonPaths := []string{
		"/rss.xml",
		"/feed.xml",
		"/atom.xml",
		"/feed",
		"/rss",
		"/feeds/posts/default", // Blogger
		"/index.xml",           // Hugo
		"/feed/",
		"/rss/",
		"/atom/",
		"/blog/feed",
		"/blog/rss",
		"/blog/feed.xml",
		"/blog/rss.xml",
		"/posts/feed",
		"/posts/rss.xml",
		"/?feed=rss2",  // WordPress
		"/feed/?type=rss", // Some WordPress
		"/rss2.xml",
		"/feed.atom",
		"/feed.rss",
	}

	// Try common paths concurrently for faster discovery
	type feedResult struct {
		url   string
		valid bool
	}
	resultCh := make(chan feedResult, len(commonPaths))

	var wg sync.WaitGroup
	semaphore := make(chan struct{}, 5) // Limit concurrent checks

	for _, path := range commonPaths {
		feedURL := baseURL + path
		wg.Add(1)
		go func(fURL string) {
			defer wg.Done()
			semaphore <- struct{}{}
			defer func() { <-semaphore }()
			
			if s.isValidFeed(ctx, fURL) {
				resultCh <- feedResult{url: fURL, valid: true}
			}
		}(feedURL)
	}

	go func() {
		wg.Wait()
		close(resultCh)
	}()

	// Return the first valid feed found
	for result := range resultCh {
		if result.valid {
			return result.url, nil
		}
	}

	return "", fmt.Errorf("RSS feed not found")
}

// isValidFeed checks if a URL is a valid RSS/Atom feed
func (s *Service) isValidFeed(ctx context.Context, feedURL string) bool {
	req, err := http.NewRequestWithContext(ctx, "HEAD", feedURL, nil)
	if err != nil {
		return false
	}

	resp, err := s.client.Do(req)
	if err != nil {
		return false
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		// Try GET if HEAD doesn't work
		req, err = http.NewRequestWithContext(ctx, "GET", feedURL, nil)
		if err != nil {
			return false
		}

		resp2, err := s.client.Do(req)
		if err != nil {
			return false
		}
		defer resp2.Body.Close()

		if resp2.StatusCode != http.StatusOK {
			return false
		}

		// Read first few bytes to check if it's XML
		buf := make([]byte, 512)
		n, err := io.ReadAtLeast(resp2.Body, buf, 1)
		if err != nil && err != io.EOF && err != io.ErrUnexpectedEOF {
			return false
		}
		if n == 0 {
			return false
		}
		content := string(buf[:n])

		// Check for XML declaration and RSS/Atom tags
		if strings.Contains(content, "<?xml") ||
			strings.Contains(content, "<rss") ||
			strings.Contains(content, "<feed") ||
			strings.Contains(content, "<atom") {
			return true
		}
		return false
	}

	contentType := resp.Header.Get("Content-Type")
	return strings.Contains(contentType, "xml") ||
		strings.Contains(contentType, "rss") ||
		strings.Contains(contentType, "atom")
}

// getFavicon gets the favicon URL for a blog
func (s *Service) getFavicon(blogURL string) string {
	u, err := url.Parse(blogURL)
	if err != nil {
		return ""
	}

	// Use Google's favicon service as fallback
	return fmt.Sprintf("https://www.google.com/s2/favicons?domain=%s", u.Host)
}
