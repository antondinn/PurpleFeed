package main

import (
	"bufio"
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"
	"gator/internal/config"
	"gator/internal/database"
	"gator/internal/rss"
	"github.com/google/uuid"
	_ "modernc.org/sqlite"
)

type state struct {
	db  *database.Queries
	cfg *config.Config
}

type command struct {
	name string
	args []string
}

type checkedPostRef struct {
	PostID   uuid.UUID
	Category string
}

type commands struct {
	coms map[string]func(*state, command) error
}

func (c *commands) run(s *state, cmd command) error {
	handler, ok := c.coms[cmd.name]
	if !ok {
		return fmt.Errorf("unknown command: %s", cmd.name)
	}
	return handler(s, cmd)
}

func (c *commands) register(name string, f func(*state, command) error) {
	c.coms[name] = f
}

func middlewareRequireDB(handler func(*state, command) error) func(*state, command) error {
	return func(s *state, cmd command) error {
		if err := requireDBConfig(s.cfg); err != nil {
			return err
		}
		return handler(s, cmd)
	}
}

func middlewareRequireObsidian(handler func(*state, command) error) func(*state, command) error {
	return func(s *state, cmd command) error {
		if err := requireObsidianConfig(s.cfg); err != nil {
			return err
		}
		return handler(s, cmd)
	}
}

func middlewareRequireDBAndObsidian(handler func(*state, command) error) func(*state, command) error {
	return func(s *state, cmd command) error {
		if err := requireDBConfig(s.cfg); err != nil {
			return err
		}
		if err := requireObsidianConfig(s.cfg); err != nil {
			return err
		}
		return handler(s, cmd)
	}
}

func requireDBConfig(cfg *config.Config) error {
	if strings.TrimSpace(cfg.DBURL) == "" {
		return errors.New("db_url is not configured")
	}
	return nil
}

func requireObsidianConfig(cfg *config.Config) error {
	if strings.TrimSpace(cfg.VaultPath) == "" {
		return errors.New("vault path is not configured")
	}
	if strings.TrimSpace(cfg.PullPath) == "" {
		return errors.New("pull note path is not configured")
	}
	if strings.TrimSpace(cfg.SavedPath) == "" {
		return errors.New("saved note path is not configured")
	}
	return nil
}

func handlerConfig(s *state, cmd command) error {
	if len(cmd.args) < 1 {
		return errors.New("usage: config <vault/pull/save/db/show> [value]")
	}

	key := strings.ToLower(strings.TrimSpace(cmd.args[0]))

	if key == "show" {
		fmt.Println("Current config:")
		fmt.Printf("	Database URL: %s\n", s.cfg.DBURL)
		fmt.Printf("	Vault path: %s\n", s.cfg.VaultPath)
		fmt.Printf("	Pulled article note path: %s\n", s.cfg.PullPath)
		fmt.Printf("	Saved article note path: %s\n", s.cfg.SavedPath)
		return nil
	}

	if len(cmd.args) < 2 {
		return errors.New("usage: config <vault/pull/save/db> <value>")
	}

	value := strings.TrimSpace(strings.Join(cmd.args[1:], " "))

	if value == "" {
		return errors.New("config value cannot be empty")
	}

	switch key {
	case "vault":
		if err := s.cfg.SetVaultPath(value); err != nil {
			return err
		}
		fmt.Printf("Obsidian vault path set to: %s\n", value)
	
	case "pull":
		if err := s.cfg.SetPullPath(value); err != nil {
			return err
		}
		fmt.Printf("Pulled feeds path set to: %s\n", value)
	
	case "save":
		if err := s.cfg.SetSavedPath(value); err != nil {
			return err
		}
		fmt.Printf("Saved feeds path set to: %s\n", value)
	case "db":
		if err := s.cfg.SetDBURL(value); err != nil {
		return err
		}
		fmt.Printf("Database URL set to: %s\n", value)
	
	default:
		return fmt.Errorf("unknown config key: %s", key)
	}

	return nil
}

func ensureSavedNoteExists(path string) error {
	_, err := os.Stat(path)
	if err == nil {
		return nil
	}
	if !os.IsNotExist(err) {
		return err
	}

	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}

	initial := "# Saved Articles\n\n"
	return os.WriteFile(path, []byte(initial), 0644)
}

func parseDate(pubDate string) time.Time {
	pubDate = strings.TrimSpace(pubDate)
	if pubDate == "" {
		return time.Now()
	}

	layouts := []string{
		time.RFC1123Z,
		time.RFC1123,
		time.RFC822Z,
		time.RFC822,
		time.RFC3339,
		time.RFC3339Nano,
		"Mon, 02 Jan 2006 15:04:05 -0700",
		"Mon, 02 Jan 2006 15:04:05 MST",
		"2006-01-02 15:04:05 -0700",
		"2006-01-02",
	}

	for _, layout := range layouts {
		t, err := time.Parse(layout, pubDate)
		if err == nil {
			return t
		}
	}

	return time.Now()
}

func MDSuffix(path string) string {
	path = strings.TrimSpace(path)
	path = strings.Trim(path, `"`)

	if path == "" {
		return path
	}

	if strings.HasSuffix(strings.ToLower(path), ".md") {
		return path
	}

	return path + ".md"
}

func buildPullNotePath(cfg *config.Config) string {
	return filepath.Join(cfg.VaultPath, MDSuffix(cfg.PullPath))
}

func buildSavedNotePath(cfg *config.Config) string {
	return filepath.Join(cfg.VaultPath, MDSuffix(cfg.SavedPath))
}

func buildPullNote(posts []database.GetPostsForActiveCategoriesRow) string {
	var b strings.Builder

	b.WriteString("# PurpleFeed Pull\n\n")
	b.WriteString(fmt.Sprintf("Pull date - %s\n\n", time.Now().Format("2006-01-02 15:04")))

	currentCategory := ""

	for _, post := range posts {
		if post.CategoryName != currentCategory {
			currentCategory = post.CategoryName
			b.WriteString(fmt.Sprintf("## %s\n\n", currentCategory))
		}

		summary := strings.TrimSpace(post.Description)
		if summary == "" {
			summary = "No summary available."
		}

		runes := []rune(summary)
		if len(runes) > 240 {
			summary = string(runes[:240]) + "..."
		}

		b.WriteString(fmt.Sprintf("- [ ] [%s](%s)\n", post.Title, post.Url))
		b.WriteString(fmt.Sprintf("  %%%% PF:post_id=%s;category=%s %%%%\n", post.ID, post.CategoryName))
		b.WriteString(fmt.Sprintf("  Source: %s\n", post.FeedName))
		b.WriteString(fmt.Sprintf("  Published: %s\n", post.PublishedAt.Format("2006-01-02 15:04")))
		b.WriteString(fmt.Sprintf("  Summary: %s\n\n", summary))
	}

	return b.String()
}

func handlerAddCategory(s *state, cmd command) error {
	if len(cmd.args) < 1 {
		return errors.New("usage: addcat <category_name>")
	}
	
	name := strings.TrimSpace(strings.Join(cmd.args, " "))
	if name == "" {
		return errors.New("Category name cannot be empty")
	}

	category, err := s.db.CreateCategory(context.Background(), database.CreateCategoryParams{
		ID: uuid.New(),
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
		Name: name,
		IsActive: true,
	})
	if err != nil {
		return fmt.Errorf("Failed to create the category: %w", err)
	}

	fmt.Printf("Added the %s category successfully", category.Name)
	fmt.Printf("Added with activity status set as %t", category.IsActive)
	return nil
}

func handlerCategories(s *state, cmd command) error {
	if len(cmd.args) != 0 {
		return errors.New("usage: cats")
	}

	categories, err := s.db.GetCategories(context.Background())
	if err != nil {
		return fmt.Errorf("Failed to get categories: %w", err)
	}

	if len(categories) == 0 {
		fmt.Println("No categories been added")
		return nil
	}

	for _, c := range categories {
		status := "off"
		if c.IsActive {
			status = "on"
		}
		fmt.Printf("%s [%s]\n", c.Name, status)
	}

	return nil
}

func handlerRemoveCategory(s *state, cmd command) error {
	if len(cmd.args) < 1 {
		return errors.New("usage: remcat <category_name>")
	}

	name := strings.TrimSpace(strings.Join(cmd.args, " "))
	if name == "" {
		return errors.New("Category name cannot be empty")
	}

	if err := s.db.DeleteCategory(context.Background(), name); err != nil {
		return fmt.Errorf("Delete failed: %w", err)
	}

	fmt.Printf("Removed the %s categoty\n", name)
	return nil
}

func handlerCategoryOn(s *state, cmd command) error {
	if len(cmd.args) < 1 {
		return errors.New("usage: caton <category_name>")
	}

	name := strings.TrimSpace(strings.Join(cmd.args, " "))
	if name == "" {
		return errors.New("Category name cannot be empty")
	}

	if err := s.db.ActivateCategory(context.Background(), name); err != nil {
		return fmt.Errorf("Failed to activate category: %w", err)
	}

	fmt.Printf("Category %s - On", name)
	return nil
}

func handlerCategoryOff(s *state, cmd command) error {
	if len(cmd.args) < 1 {
		return errors.New("usage: catoff <category_name>")
	}

	name := strings.TrimSpace(strings.Join(cmd.args, " "))
	if name == "" {
		return errors.New("category name cannot be empty")
	}

	if err := s.db.DeactivateCategory(context.Background(), name); err != nil {
		return fmt.Errorf("failed to turn off the category: %w", err)
	}

	fmt.Printf("Category %s was deactivated successfully\n", name)
	return nil
}

func handlerAddFeeds(s *state, cmd command) error {
	if len(cmd.args) < 1 {
		return errors.New("usage: addfeeds <category_name>")
	}

	categoryName := strings.TrimSpace(strings.Join(cmd.args, " "))
	if categoryName == "" {
		return errors.New("category name cannot be empty")
	}

	category, err := s.db.GetCategory(context.Background(), categoryName)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return fmt.Errorf("category %s could not be found", categoryName)
		}
		return fmt.Errorf("failed to locate category %s: %w", categoryName, err)
	}

	fmt.Printf("Adding feeds to category: %s\n", category.Name)
	fmt.Println(`Paste feed URLs one by one. Type "end" when done.`)

	scanner := bufio.NewScanner(os.Stdin)

	for {
		fmt.Print("> ")
		if !scanner.Scan() {
			if err := scanner.Err(); err != nil {
				return err
			}
			break
		}

		input := strings.TrimSpace(scanner.Text())
		if input == "" {
			continue
		}

		if strings.EqualFold(input, "end") {
			break
		}

		feedURL := input
		_, err := s.db.GetFeedByURL(context.Background(), feedURL)
		if err == nil {
			fmt.Println("Feed already exists")
			continue
		}
		if !errors.Is(err, sql.ErrNoRows) {
			return fmt.Errorf("failed duplicate feed check: %w", err)
		}

		fetchedFeed, err := rss.FetchFeed(context.Background(), feedURL)
		if err != nil {
			fmt.Printf("Feed couldn't be reached: %v\n", err)
			continue
		}

		feedName := strings.TrimSpace(fetchedFeed.Channel.Title)
		if feedName == "" {
			feedName = feedURL
		}

		createdFeed, err := s.db.CreateFeed(context.Background(), database.CreateFeedParams{
			ID:            uuid.New(),
			CreatedAt:     time.Now(),
			UpdatedAt:     time.Now(),
			Name:          feedName,
			Url:           feedURL,
			CategoryID:    category.ID,
			LastFetchedAt: sql.NullTime{},
		})
		if err != nil {
			fmt.Printf("Failed to add feed: %v\n", err)
			continue
		}

		fmt.Printf("Successfully added the feed %s to the %s category\n", createdFeed.Name, categoryName)
	}
	return nil
}

func handlerRemoveFeeds(s *state, cmd command) error {
	if len(cmd.args) < 1 {
		return errors.New("usage: remfeeds <category_name>")
	}

	categoryName := strings.TrimSpace(strings.Join(cmd.args, " "))
	if categoryName == "" {
		return errors.New("category name cannot be empty")
	}

	_, err := s.db.GetCategory(context.Background(), categoryName)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return fmt.Errorf("category %s doesn't exist", categoryName)
		}
		return fmt.Errorf("failed to get the category: %w", err)
	}

	scanner := bufio.NewScanner(os.Stdin)

	for {
		feeds, err := s.db.GetFeedsByCategoryName(context.Background(), categoryName)
		if err != nil {
			return fmt.Errorf("failed to get the feeds for the category: %w", err)
		}

		if len(feeds) == 0 {
			fmt.Printf("No feeds found under the %s category\n", categoryName)
			return nil
		}

		fmt.Printf("The feeds under %s category:\n", categoryName)
		for i, feed := range feeds {
			fmt.Printf("%d. %s - %s\n", i+1, feed.Name, feed.Url)
		}

		fmt.Println(`Enter the number of the feed to remove, or type "end" to stop`)
		fmt.Print("> ")

		if !scanner.Scan() {
			if err := scanner.Err(); err != nil {
				return err
			}
			return nil
		}

		input := strings.TrimSpace(scanner.Text())
		if input == "" {
			continue
		}

		if strings.EqualFold(input, "end") {
			fmt.Println()
			return nil
		}

		chosenNumber, err := strconv.Atoi(input)
		if err != nil {
			fmt.Println("The choice was not a valid number")
			continue
		}

		if chosenNumber < 1 || chosenNumber > len(feeds) {
			fmt.Println("Number out of range")
			continue
		}

		chosenFeed := feeds[chosenNumber-1]

		if err := s.db.DeleteFeedById(context.Background(), chosenFeed.ID); err != nil {
			return fmt.Errorf("failed to delete feed: %w", err)
		}

		fmt.Printf("Removed the %s (%s) from the %s category\n", chosenFeed.Name, chosenFeed.Url, categoryName)
	}
}

func limitPostsPerCategory(posts []database.GetPostsForActiveCategoriesRow, limit int) []database.GetPostsForActiveCategoriesRow {
	if limit <= 0 {
		return posts
	}

	counts := make(map[string]int)
	limited := make([]database.GetPostsForActiveCategoriesRow, 0, len(posts))

	for _, post := range posts {
		if counts[post.CategoryName] >= limit {
			continue
		}
		counts[post.CategoryName]++
		limited = append(limited, post)
	}

	return limited
}

func handlerPullPurp(s *state, cmd command) error {
	limitPerCategory := int32(10)

	if len(cmd.args) > 1 {
		return errors.New("usage: pullpurp [limit_per_category]")
	}
	if len(cmd.args) == 1 {
		n, err := strconv.Atoi(strings.TrimSpace(cmd.args[0]))
		if err != nil || n <= 0 {
			return errors.New("limit must be a positive integer")
		}
		limitPerCategory = int32(n)
	}

	feeds, err := s.db.GetFeedsForActiveCategories(context.Background())
	if err != nil {
		return fmt.Errorf("failed to get feeds for active categories: %w", err)
	}

	if len(feeds) == 0 {
		fmt.Println("No feeds found for active categories.")
		return nil
	}

	pulledPostCount := 0

	for _, feed := range feeds {
		rssFeed, err := rss.FetchFeed(context.Background(), feed.Url)
		if err != nil {
			fmt.Printf("Skipping feed - %s - %s: %v\n", feed.Name, feed.Url, err)
			continue
		}

		for _, item := range rssFeed.Channel.Item {
			title := strings.TrimSpace(item.Title)
			url := strings.TrimSpace(item.Link)
			description := strings.TrimSpace(rss.ExtractBestSummary(item))

			if title == "" || url == "" {
				continue
			}

			err := s.db.CreatePost(context.Background(), database.CreatePostParams{
				ID:          uuid.New(),
				CreatedAt:   time.Now(),
				UpdatedAt:   time.Now(),
				Title:       title,
				Url:         url,
				Description: description,
				PublishedAt: parseDate(item.PubDate),
				FeedID:      feed.ID,
			})
			if err != nil {
				fmt.Printf("Failed to save post from %s: %v\n", feed.Name, err)
				continue
			}

			pulledPostCount++
		}

		if err := s.db.MarkFeedFetched(context.Background(), feed.ID); err != nil {
			fmt.Printf("Failed to mark feed as fetched for %s: %v\n", feed.Name, err)
		}
	}

	posts, err := s.db.GetPostsForActiveCategories(context.Background())
	if err != nil {
		return fmt.Errorf("failed to get posts for active categories: %w", err)
	}

	posts = limitPostsPerCategory(posts, int(limitPerCategory))

	if len(posts) == 0 {
		fmt.Println("No posts found for active categories.")
		return nil
	}

	pullPath := buildPullNotePath(s.cfg)

	if err := os.MkdirAll(filepath.Dir(pullPath), 0755); err != nil {
		return fmt.Errorf("failed to create Pull note directory: %w", err)
	}

	content := buildPullNote(posts)

	if err := os.WriteFile(pullPath, []byte(content), 0644); err != nil {
		return fmt.Errorf("failed to write Pull note: %w", err)
	}

	fmt.Printf("Pull complete. %d new posts saved.\n", pulledPostCount)
	fmt.Printf("Pull note written to: %s\n", pullPath)
	return nil
}

func parseMetaData(metadata string) (checkedPostRef, error) {
	metadata = strings.TrimSpace(metadata)

	const prefix = "%% PF:"
	const suffix = "%%"

	if !strings.HasPrefix(metadata, prefix) || !strings.HasSuffix(metadata, suffix) {
		return checkedPostRef{}, errors.New("invalid PurpleFeed metadata format")
	}

	content := strings.TrimSpace(strings.TrimSuffix(strings.TrimPrefix(metadata, prefix), suffix))
	halves := strings.Split(content, ";")

	values := make(map[string]string)
	for _, half := range halves {
		half = strings.TrimSpace(half)
		if half == "" {
			continue
		}

		kv := strings.SplitN(half, "=", 2)
		if len(kv) != 2 {
			continue
		}

		key := strings.TrimSpace(kv[0])
		value := strings.TrimSpace(kv[1])
		values[key] = value
	}

	idStr, ok := values["post_id"]
	if !ok || idStr == "" {
		return checkedPostRef{}, errors.New("missing metadata value: post_id")
	}

	idVal, err := uuid.Parse(idStr)
	if err != nil {
		return checkedPostRef{}, fmt.Errorf("post ID is invalid: %w", err)
	}

	return checkedPostRef{
		PostID:   idVal,
		Category: values["category"],
	}, nil
}

func parseCheckboxes(content string) ([]checkedPostRef, error) {
	lines := strings.Split(content, "\n")
	refs := make([]checkedPostRef, 0)

	for i:=0; i < len(lines); i++ {
		line := strings.TrimSpace(lines[i])

		if strings.HasPrefix(line, "- [x] ") || strings.HasPrefix(line, "- [X] ") {
			if i+1 >= len(lines) {
				return nil, errors.New("Metadata missing")
			}

			metadataLine := strings.TrimSpace(lines[i+1])
			ref, err := parseMetaData(metadataLine)
			if err != nil {
				return nil, fmt.Errorf("Failed to pars metadata: line %d %w", i+2, err)
			}

			refs = append(refs,ref)
		}
	}
	return refs, nil
}

func renderSavedEntry(post database.Post, category string, feedName string) string {
	summary := strings.TrimSpace(post.Description)
	if summary == "" {
		summary = "No summary available."
	}

	runes := []rune(summary)
	if len(runes) > 240 {
		summary = string(runes[:240]) + "..."
	}

	feedName = strings.TrimSpace(feedName)
	if feedName == "" {
		feedName = "Unknown source"
	}

	var b strings.Builder
	b.WriteString(fmt.Sprintf("- [ ] [%s](%s)\n", post.Title, post.Url))
	b.WriteString(fmt.Sprintf("  %%%% PF:post_id=%s;category=%s %%%%\n", post.ID, category))
	b.WriteString(fmt.Sprintf("  Source: %s\n", feedName))
	b.WriteString(fmt.Sprintf("  Saved from: %s\n", time.Now().Format("2006-01-02")))
	b.WriteString(fmt.Sprintf("  Published: %s\n", post.PublishedAt.Format("2006-01-02 15:04")))
	b.WriteString(fmt.Sprintf("  Summary: %s\n\n", summary))
	return b.String()
}

func buildSavedEntriesByCategory(
	posts []database.Post,
	categoryByPostID map[uuid.UUID]string,
	feedNameByPostID map[uuid.UUID]string,
) map[string]string {
	grouped := make(map[string]string)

	for _, post := range posts {
		category := strings.TrimSpace(categoryByPostID[post.ID])
		if category == "" {
			category = "Uncategorized"
		}

		entry := renderSavedEntry(post, category, feedNameByPostID[post.ID])
		grouped[category] += entry
	}

	return grouped
}

func insertSavedEntriesIntoNote(existing string, entriesByCategory map[string]string) string {
	existing = strings.ReplaceAll(existing, "\r\n", "\n")

	if strings.TrimSpace(existing) == "" {
		existing = "# Saved Articles\n\n"
	}

	lines := strings.Split(existing, "\n")

	sectionStart := make(map[string]int)
	sectionEnd := make(map[string]int)
	order := make([]string, 0)

	for i := 0; i < len(lines); i++ {
		line := strings.TrimSpace(lines[i])
		if strings.HasPrefix(line, "## ") {
			category := strings.TrimSpace(strings.TrimPrefix(line, "## "))
			if category == "" {
				continue
			}

			sectionStart[category] = i
			order = append(order, category)

			j := i + 1
			for j < len(lines) {
				next := strings.TrimSpace(lines[j])
				if strings.HasPrefix(next, "## ") {
					break
				}
				j++
			}
			sectionEnd[category] = j
		}
	}

	newCategories := make([]string, 0)
	for category := range entriesByCategory {
		if _, exists := sectionStart[category]; !exists {
			newCategories = append(newCategories, category)
		}
	}

	sort.Strings(newCategories)

	var b strings.Builder
	i := 0

	for i < len(lines) {
		line := lines[i]
		trimmed := strings.TrimSpace(line)

		if strings.HasPrefix(trimmed, "## ") {
			category := strings.TrimSpace(strings.TrimPrefix(trimmed, "## "))
			end := sectionEnd[category]

			sectionText := strings.Join(lines[i:end], "\n")
			b.WriteString(sectionText)

			if !strings.HasSuffix(sectionText, "\n") {
				b.WriteString("\n")
			}

			if !strings.HasSuffix(sectionText, "\n\n") {
				b.WriteString("\n")
			}

			if extra, ok := entriesByCategory[category]; ok {
				b.WriteString(extra)
				delete(entriesByCategory, category)
			}

			i = end
			continue
		}

		b.WriteString(line)
		if i < len(lines)-1 {
			b.WriteString("\n")
		}
		i++
	}

	if len(lines) > 0 && !strings.HasSuffix(b.String(), "\n") {
		b.WriteString("\n")
	}

	if !strings.HasSuffix(b.String(), "\n\n") {
		b.WriteString("\n")
	}

	for _, category := range newCategories {
		b.WriteString(fmt.Sprintf("## %s\n\n", category))
		b.WriteString(entriesByCategory[category])
		delete(entriesByCategory, category)
	}

	return b.String()
}

func handlerSavePurp(s *state, cmd command) error {
	if len(cmd.args) != 0 {
		return errors.New("usage: savepurp")
	}

	pullPath := buildPullNotePath(s.cfg)
	savedPath := buildSavedNotePath(s.cfg)

	pullData, err := os.ReadFile(pullPath)
	if err != nil {
		if os.IsNotExist(err) {
			return errors.New("pull note doesn't exist")
		}
		return fmt.Errorf("failed to read the pull note: %w", err)
	}

	refs, err := parseCheckboxes(string(pullData))
	if err != nil {
		return fmt.Errorf("failed to parse checked articles from the pull note: %w", err)
	}

	if len(refs) == 0 {
		return errors.New("no checked articles in pull note")
	}

	ctx := context.Background()

	posts := make([]database.Post, 0, len(refs))
	categoryByPostID := make(map[uuid.UUID]string)
	feedNameByPostID := make(map[uuid.UUID]string)

	for _, ref := range refs {
		post, err := s.db.GetPostByID(ctx, ref.PostID)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				fmt.Printf("Skipping missing post %s\n", ref.PostID)
				continue
			}
			return fmt.Errorf("failed to get post %s: %w", ref.PostID, err)
		}

		feed, err := s.db.GetFeedByID(ctx, post.FeedID)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				feedNameByPostID[post.ID] = "Unknown source"
			} else {
				return fmt.Errorf("failed to get feed for post %s: %w", post.ID, err)
			}
		} else {
			feedNameByPostID[post.ID] = feed.Name
		}

		posts = append(posts, post)
		categoryByPostID[post.ID] = ref.Category
	}

	if len(posts) == 0 {
		return errors.New("no valid checked posts found in pull note")
	}

	if err := ensureSavedNoteExists(savedPath); err != nil {
		return fmt.Errorf("failed to prepare saved note: %w", err)
	}

	existingSaved, err := os.ReadFile(savedPath)
	if err != nil {
		return fmt.Errorf("failed to read saved note: %w", err)
	}

	entriesByCategory := buildSavedEntriesByCategory(posts, categoryByPostID, feedNameByPostID)
	updatedSavedNote := insertSavedEntriesIntoNote(string(existingSaved), entriesByCategory)

	if err := os.WriteFile(savedPath, []byte(updatedSavedNote), 0644); err != nil {
		return fmt.Errorf("failed to rewrite saved note: %w", err)
	}

	if err := os.Remove(pullPath); err != nil {
		return fmt.Errorf("saved successfully, but failed to delete pull note: %w", err)
	}

	fmt.Printf("Saved %d article(s) to: %s\n", len(posts), savedPath)
	fmt.Printf("Deleted pull note: %s\n", pullPath)
	return nil
}

func isSavedArticleStart(line string) bool {
	line = strings.TrimSpace(line)
	return strings.HasPrefix(line, "- [ ] ") ||
		strings.HasPrefix(line, "- [x] ") ||
		strings.HasPrefix(line, "- [X] ")
}

func isCheckedArticleLine(line string) bool {
	line = strings.TrimSpace(line)
	return strings.HasPrefix(line, "- [x] ") || strings.HasPrefix(line, "- [X] ")
}

func removeCheckedSavedBlocks(content string) (string, int) {
	content = strings.ReplaceAll(content, "\r\n", "\n")
	lines := strings.Split(content, "\n")

	out := make([]string, 0, len(lines))
	removedCount := 0

	for i := 0; i < len(lines); {
		line := lines[i]

		if isSavedArticleStart(line) {
			start := i
			checked := isCheckedArticleLine(line)

			i++

			for i < len(lines) {
				next := strings.TrimSpace(lines[i])

				if strings.HasPrefix(next, "## ") {
					break
				}
				if isSavedArticleStart(lines[i]) {
					break
				}

				i++
			}
			if checked {
				removedCount++
				continue
			}
			out = append(out, lines[start:i]...)
			continue
		}
		out = append(out, line)
		i++
	}
	return strings.Join(out, "\n"), removedCount
}

func removeEmptyCategorySections(content string) string {
	content = strings.ReplaceAll(content, "\r\n", "\n")
	lines := strings.Split(content, "\n")

	out := make([]string, 0, len(lines))

	for i := 0; i < len(lines); {
		line := lines[i]
		trimmed := strings.TrimSpace(line)

		if strings.HasPrefix(trimmed, "## ") {
			start := i
			i++

			hasArticle := false
			for i < len(lines) {
				nextTrimmed := strings.TrimSpace(lines[i])

				if strings.HasPrefix(nextTrimmed, "## ") {
					break
				}
				if isSavedArticleStart(lines[i]) {
					hasArticle = true
				}

				i++
			}
			if hasArticle {
				out = append(out, lines[start:i]...)
			}
			continue
		}
		out = append(out, line)
		i++
	}
	result := strings.Join(out, "\n")
	return normalizeSavedNoteSpacing(result)
}

func normalizeSavedNoteSpacing(content string) string {
	content = strings.ReplaceAll(content, "\r\n", "\n")
	lines := strings.Split(content, "\n")

	out := make([]string, 0, len(lines))
	prevBlank := false

	for _, line := range lines {
		isBlank := strings.TrimSpace(line) == ""

		if isBlank && prevBlank {
			continue
		}

		out = append(out, line)
		prevBlank = isBlank
	}

	result := strings.Join(out, "\n")
	result = strings.TrimSpace(result)

	if result == "" {
		return "# Saved Articles\n"
	}

	if !strings.HasPrefix(result, "# Saved Articles") {
		result = "# Saved Articles\n\n" + result
	}
	return result + "\n"
}

func handlerCleanPurp(s *state, cmd command) error {
	if len(cmd.args) != 0 {
		return errors.New("usage: delpurp")
	}

	savedPath := buildSavedNotePath(s.cfg)

	data, err := os.ReadFile(savedPath)
	if err != nil {
		if os.IsNotExist(err) {
			return errors.New("saved note does not exist")
		}
		return fmt.Errorf("failed to read saved note: %w", err)
	}

	afterBlockRemoval, removedCount := removeCheckedSavedBlocks(string(data))
	finalContent := removeEmptyCategorySections(afterBlockRemoval)
	finalContent = normalizeSavedNoteSpacing(finalContent)

	if err := os.WriteFile(savedPath, []byte(finalContent), 0644); err != nil {
		return fmt.Errorf("failed to rewrite saved note: %w", err)
	}

	fmt.Printf("Removed %d checked article(s) from: %s\n", removedCount, savedPath)
	return nil
}

func handlerHelp(s *state, cmd command) error {
	if len(cmd.args) != 0 {
		return errors.New("usage: help")
	}

	fmt.Println("Available commands:")
	fmt.Println("  help")
	fmt.Println("  config")
	fmt.Println("  addcat")
	fmt.Println("  cats")
	fmt.Println("  remcat")
	fmt.Println("  caton")
	fmt.Println("  catoff")
	fmt.Println("  addfeeds")
	fmt.Println("  remfeeds")
	fmt.Println("  feeds")
	fmt.Println("  pullpurp")
	fmt.Println("  savepurp")
	fmt.Println("  cleanpurp")

	return nil
}
	
func main() {

	cfg, err := config.Read()
	if err != nil {
		log.Fatal(err)
	}

	if len(os.Args) < 2 {
		fmt.Fprintln(os.Stderr, "not enough arguments")
		os.Exit(1)
	}

	cmd := command{
		name: os.Args[1],
		args: os.Args[2:],
	}

	s := &state{
		cfg: &cfg,
	}

	needsDB := map[string]bool{
		"addcat":   true,
		"cats":     true,
		"remcat":   true,
		"caton":    true,
		"catoff":   true,
		"addfeeds": true,
		"remfeeds": true,
		"pullpurp": true,
		"savepurp": true,
	}

	if needsDB[cmd.name] {
		db, err := sql.Open("sqlite", cfg.DBURL)
		if err != nil {
			log.Fatal(err)
		}

		db.SetMaxOpenConns(1)

		if _, err := db.Exec("PRAGMA foreign_keys = ON"); err != nil {
			log.Fatal(err)
		} 
		s.db = database.New(db)
	}

	cmds := commands{coms: make(map[string]func(*state, command) error)}
	cmds.register("config", handlerConfig)
	cmds.register("addcat", middlewareRequireDB(handlerAddCategory))
	cmds.register("cats", middlewareRequireDB(handlerCategories))
	cmds.register("remcat", middlewareRequireDB(handlerRemoveCategory))
	cmds.register("caton", middlewareRequireDB(handlerCategoryOn))
	cmds.register("catoff", middlewareRequireDB(handlerCategoryOff))
	cmds.register("addfeeds", middlewareRequireDB(handlerAddFeeds))
	cmds.register("remfeeds", middlewareRequireDB(handlerRemoveFeeds))
	cmds.register("pullpurp", middlewareRequireDBAndObsidian(handlerPullPurp))
	cmds.register("savepurp", middlewareRequireDBAndObsidian(handlerSavePurp))
	cmds.register("cleanpurp", middlewareRequireObsidian(handlerCleanPurp))
	cmds.register("help", handlerHelp)

	if err := cmds.run(s, cmd); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

/*postgres://postgres:postgres@localhost:5433/gator*/