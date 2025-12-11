package utils

import (
	"regexp"
	"strings"
)

// selfClosingTags is the list of HTML self-closing tags to handle
const selfClosingTags = "img|br|hr|input|meta|link"

// Compile regex patterns once at package initialization for better performance
var (
	// Matches malformed opening tags like <p-->, <div-->
	malformedTagRegex = regexp.MustCompile(`<([a-zA-Z][a-zA-Z0-9]*)\s*--+>`)

	// Matches malformed self-closing tags with attributes like <img src="..." -->
	malformedSelfClosingWithAttrs = regexp.MustCompile(`<(` + selfClosingTags + `)\s+([^<>]+?)--+>`)

	// Matches malformed self-closing tags without attributes like <br-->
	malformedSelfClosingNoAttrs = regexp.MustCompile(`<(` + selfClosingTags + `)\s*--+>`)

	// Matches style attributes in HTML tags
	styleAttrRegex = regexp.MustCompile(`\s+style\s*=\s*"[^"]*"`)

	// Alternative style attribute with single quotes
	styleAttrSingleQuoteRegex = regexp.MustCompile(`\s+style\s*=\s*'[^']*'`)

	// Matches class attributes in HTML tags
	classAttrRegex = regexp.MustCompile(`\s+class\s*=\s*"[^"]*"`)

	// Alternative class attribute with single quotes
	classAttrSingleQuoteRegex = regexp.MustCompile(`\s+class\s*=\s*'[^']*'`)

	// Matches <style> tags and their content
	styleTagRegex = regexp.MustCompile(`(?i)<style[^>]*>.*?</style>`)

	// Matches <script> tags and their content
	scriptTagRegex = regexp.MustCompile(`(?i)<script[^>]*>.*?</script>`)
)

// CleanHTML sanitizes HTML content by fixing common malformed patterns
// and removing unwanted inline styles, classes, and scripts.
func CleanHTML(html string) string {
	if html == "" {
		return html
	}

	// Fix malformed opening tags like <p--> to <p>
	// This pattern matches tags like <p-->, <div-->, etc.
	html = malformedTagRegex.ReplaceAllString(html, "<$1>")

	// Fix malformed self-closing tags like <img-->, <br--> to <img>, <br>
	// Some feeds have broken self-closing tags with or without attributes
	// Pattern 1: Tags with attributes (e.g., <img src="..." -->)
	// Use [^<>]+ to avoid matching angle brackets and nested tags
	html = malformedSelfClosingWithAttrs.ReplaceAllString(html, "<$1 $2>")

	// Pattern 2: Tags without attributes (e.g., <br-->)
	html = malformedSelfClosingNoAttrs.ReplaceAllString(html, "<$1>")

	// Remove inline style attributes (both double and single quotes)
	html = styleAttrRegex.ReplaceAllString(html, "")
	html = styleAttrSingleQuoteRegex.ReplaceAllString(html, "")

	// Remove class attributes (both double and single quotes)
	html = classAttrRegex.ReplaceAllString(html, "")
	html = classAttrSingleQuoteRegex.ReplaceAllString(html, "")

	// Remove <style> tags and their content
	html = styleTagRegex.ReplaceAllString(html, "")

	// Remove <script> tags and their content
	html = scriptTagRegex.ReplaceAllString(html, "")

	// Trim any leading/trailing whitespace
	html = strings.TrimSpace(html)

	return html
}
