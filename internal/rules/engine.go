package rules

import (
	"encoding/json"
	"log"
	"strings"
	"time"

	"MrRSS/internal/database"
	"MrRSS/internal/models"
)

// Condition represents a condition in a rule
type Condition struct {
	ID       int64    `json:"id"`
	Logic    string   `json:"logic"`    // "and", "or" (null for first condition)
	Negate   bool     `json:"negate"`   // NOT modifier for this condition
	Field    string   `json:"field"`    // "feed_name", "feed_category", "article_title", etc.
	Operator string   `json:"operator"` // "contains", "exact"
	Value    string   `json:"value"`    // Single value for text/date fields
	Values   []string `json:"values"`   // Multiple values for feed_name and feed_category
}

// Rule represents an automation rule
type Rule struct {
	ID         int64       `json:"id"`
	Name       string      `json:"name"`
	Enabled    bool        `json:"enabled"`
	Conditions []Condition `json:"conditions"`
	Actions    []string    `json:"actions"` // "favorite", "unfavorite", "hide", "unhide", "mark_read", "mark_unread"
}

// Engine handles rule application
type Engine struct {
	db *database.DB
}

// NewEngine creates a new rules engine
func NewEngine(db *database.DB) *Engine {
	return &Engine{db: db}
}

// ApplyRulesToArticles applies all enabled rules to a batch of articles.
// Each article is matched against rules in order, and only the first matching rule is applied.
// This prevents conflicting actions from multiple rules being applied to the same article.
func (e *Engine) ApplyRulesToArticles(articles []models.Article) (int, error) {
	// Load rules from settings
	rulesJSON, _ := e.db.GetSetting("rules")
	if rulesJSON == "" {
		return 0, nil
	}

	var rules []Rule
	if err := json.Unmarshal([]byte(rulesJSON), &rules); err != nil {
		log.Printf("Error parsing rules: %v", err)
		return 0, err
	}

	// Get feeds for category and title lookup
	feeds, err := e.db.GetFeeds()
	if err != nil {
		return 0, err
	}

	// Create maps of feed ID to category and title
	feedCategories := make(map[int64]string)
	feedTitles := make(map[int64]string)
	for _, feed := range feeds {
		feedCategories[feed.ID] = feed.Category
		feedTitles[feed.ID] = feed.Title
	}

	affected := 0
	for _, article := range articles {
		for _, rule := range rules {
			if !rule.Enabled {
				continue
			}

			// Check if article matches conditions
			if matchesConditions(article, rule.Conditions, feedCategories, feedTitles) {
				// Apply actions
				for _, action := range rule.Actions {
					if err := e.applyAction(article.ID, action); err != nil {
						log.Printf("Error applying action %s to article %d: %v", action, article.ID, err)
						continue
					}
				}
				affected++
				break // Only apply first matching rule per article to prevent conflicts
			}
		}
	}

	return affected, nil
}

// ApplyRule applies a single rule to all matching articles.
// Uses batch processing with a reasonable limit to avoid memory issues.
func (e *Engine) ApplyRule(rule Rule) (int, error) {
	// Get articles in batches to avoid memory issues with large datasets
	const batchSize = 10000
	articles, err := e.db.GetArticles("", 0, "", true, batchSize, 0)
	if err != nil {
		return 0, err
	}

	// Get feeds for category and title lookup
	feeds, err := e.db.GetFeeds()
	if err != nil {
		return 0, err
	}

	// Create maps of feed ID to category and title
	feedCategories := make(map[int64]string)
	feedTitles := make(map[int64]string)
	for _, feed := range feeds {
		feedCategories[feed.ID] = feed.Category
		feedTitles[feed.ID] = feed.Title
	}

	affected := 0
	for _, article := range articles {
		if matchesConditions(article, rule.Conditions, feedCategories, feedTitles) {
			for _, action := range rule.Actions {
				if err := e.applyAction(article.ID, action); err != nil {
					log.Printf("Error applying action %s to article %d: %v", action, article.ID, err)
					continue
				}
			}
			affected++
		}
	}

	return affected, nil
}

// matchesConditions checks if an article matches the rule conditions
func matchesConditions(article models.Article, conditions []Condition, feedCategories map[int64]string, feedTitles map[int64]string) bool {
	// If no conditions, apply to all articles
	if len(conditions) == 0 {
		return true
	}

	result := evaluateCondition(article, conditions[0], feedCategories, feedTitles)

	for i := 1; i < len(conditions); i++ {
		condition := conditions[i]
		conditionResult := evaluateCondition(article, condition, feedCategories, feedTitles)

		switch condition.Logic {
		case "and":
			result = result && conditionResult
		case "or":
			result = result || conditionResult
		}
	}

	return result
}

// evaluateCondition evaluates a single rule condition
func evaluateCondition(article models.Article, condition Condition, feedCategories map[int64]string, feedTitles map[int64]string) bool {
	var result bool

	switch condition.Field {
	case "feed_name":
		feedTitle := feedTitles[article.FeedID]
		if feedTitle == "" {
			feedTitle = article.FeedTitle
		}
		result = matchMultiSelect(feedTitle, condition.Values, condition.Value)

	case "feed_category":
		feedCategory := feedCategories[article.FeedID]
		result = matchMultiSelect(feedCategory, condition.Values, condition.Value)

	case "article_title":
		if condition.Value == "" {
			result = true
		} else {
			lowerValue := strings.ToLower(condition.Value)
			lowerTitle := strings.ToLower(article.Title)
			if condition.Operator == "exact" {
				result = lowerTitle == lowerValue
			} else {
				result = strings.Contains(lowerTitle, lowerValue)
			}
		}

	case "published_after":
		if condition.Value == "" {
			result = true
		} else {
			afterDate, err := time.Parse("2006-01-02", condition.Value)
			if err != nil {
				result = true
			} else {
				result = article.PublishedAt.After(afterDate) || article.PublishedAt.Equal(afterDate)
			}
		}

	case "published_before":
		if condition.Value == "" {
			result = true
		} else {
			beforeDate, err := time.Parse("2006-01-02", condition.Value)
			if err != nil {
				result = true
			} else {
				articleDateOnly := article.PublishedAt.UTC().Truncate(24 * time.Hour)
				beforeDateOnly := beforeDate.Truncate(24 * time.Hour)
				result = !articleDateOnly.After(beforeDateOnly)
			}
		}

	case "is_read":
		if condition.Value == "" {
			result = true
		} else {
			wantRead := condition.Value == "true"
			result = article.IsRead == wantRead
		}

	case "is_favorite":
		if condition.Value == "" {
			result = true
		} else {
			wantFavorite := condition.Value == "true"
			result = article.IsFavorite == wantFavorite
		}

	case "is_hidden":
		if condition.Value == "" {
			result = true
		} else {
			wantHidden := condition.Value == "true"
			result = article.IsHidden == wantHidden
		}

	default:
		result = true
	}

	// Apply NOT modifier
	if condition.Negate {
		return !result
	}
	return result
}

// matchMultiSelect checks if fieldValue matches any of the selected values
func matchMultiSelect(fieldValue string, values []string, singleValue string) bool {
	if len(values) > 0 {
		lowerField := strings.ToLower(fieldValue)
		for _, val := range values {
			if strings.Contains(lowerField, strings.ToLower(val)) {
				return true
			}
		}
		return false
	} else if singleValue != "" {
		return strings.Contains(strings.ToLower(fieldValue), strings.ToLower(singleValue))
	}
	return true
}

// applyAction applies an action to an article
func (e *Engine) applyAction(articleID int64, action string) error {
	switch action {
	case "favorite":
		return e.db.SetArticleFavorite(articleID, true)
	case "unfavorite":
		return e.db.SetArticleFavorite(articleID, false)
	case "hide":
		return e.db.SetArticleHidden(articleID, true)
	case "unhide":
		return e.db.SetArticleHidden(articleID, false)
	case "mark_read":
		return e.db.MarkArticleRead(articleID, true)
	case "mark_unread":
		return e.db.MarkArticleRead(articleID, false)
	default:
		log.Printf("Unknown action: %s", action)
		return nil
	}
}
