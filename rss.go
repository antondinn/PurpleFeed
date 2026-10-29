package rss

import (
	"context"
	"fmt"
	"net/http"
	"encoding/xml"
	"regexp"
	"time"
	"io"
	"html"
	"strings"
)

type RSSFeed struct {
	Channel struct {
		Title       string    `xml:"title"`
		Link        string    `xml:"link"`
		Description string    `xml:"description"`
		Item        []RSSItem `xml:"item"`
	} `xml:"channel"`
}

type RSSItem struct {
	Title       string `xml:"title"`
	Link        string `xml:"link"`
	Description string `xml:"description"`
	ContentEncoded string `xml:"encoded"`
	PubDate     string `xml:"pubDate"`
}

var htmlTagRegex = regexp.MustCompile(`<[^>]*>`)

func blockedHTML(contentType, body string) bool {
	if strings.Contains(contentType, "text/html") {
		return true
	}

	if strings.HasPrefix(body, "<!doctype html") || strings.HasPrefix(body, "<html") {
		return true
	}

	blockSigns := []string{
		"just a moment",
		"cloudflare",
		"captcha",
		"access denied",
		"bot verification",
		"checking your browser",
	}


	for _, sign := range blockSigns {
		if strings.Contains(body, sign) {
			return true
		}
	}

	return false
}

func cleanSummaryText(s string) string{
	s = html.UnescapeString(s)
	s = htmlTagRegex.ReplaceAllString(s, " ")
	s = strings.ReplaceAll(s, "\n", " ")
	s = strings.ReplaceAll(s, "\r", " ")
	s = strings.ReplaceAll(s, "\t", " ")
	s = strings.Join(strings.Fields(s), " ")
	return strings.TrimSpace(s)
}

func ExtractBestSummary(item RSSItem) string {
	candidates := []string{
		item.Description,
		item.ContentEncoded,
	}

	for _, candidate := range candidates {
		cleaned := cleanSummaryText(candidate)
		if cleaned != "" {
			return cleaned
		}
	}

	return ""
}

func FetchFeed(ctx context.Context, feedURL string) (*RSSFeed, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", feedURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("User-Agent", "Mozilla/5.0")
	req.Header.Set("Accept", "application/rss+xml, application/xml, text/xml;q=0.9, */*;q=0.8")
	req.Header.Set("Accept-Language", "en-US,en;q=0.9")

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch feed: %w", err)
	}
	defer resp.Body.Close()
	
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	contentType := strings.ToLower(resp.Header.Get("Content-Type"))
	trimmedBody := strings.TrimSpace(strings.ToLower(string(body)))

	if blockedHTML(contentType, trimmedBody) {
		preview := body
		if len(preview) > 500 {
			preview = preview[:500]
		}
		return nil, fmt.Errorf("feed appears to be blocked by anti-bot protection or is serving HTML instead of RSS\n> %s\nResponse preview:\n%s", feedURL, string(preview),)
	}
	
	var feed RSSFeed
	if err := xml.Unmarshal(body, &feed); err != nil {
		preview := body
		if len(preview) > 500 {
			preview = preview[:500]
		}
		fmt.Println("Response preview:")
		fmt.Println(string(preview))
		return nil, fmt.Errorf("failed to parse feed: %w", err)
	}

	feed.Channel.Title = html.UnescapeString(feed.Channel.Title)
	feed.Channel.Description = html.UnescapeString(feed.Channel.Description)
	for i := range feed.Channel.Item {
		feed.Channel.Item[i].Title = html.UnescapeString(feed.Channel.Item[i].Title)
		feed.Channel.Item[i].Description = html.UnescapeString(feed.Channel.Item[i].Description)
		feed.Channel.Item[i].ContentEncoded = html.UnescapeString(feed.Channel.Item[i].ContentEncoded)
	}

	return &feed, nil
}

