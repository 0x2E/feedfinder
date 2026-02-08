package feedfinder

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"net/url"

	"github.com/mmcdole/gofeed"
)

func (f *Finder) tryWellKnown(ctx context.Context, baseURL string) ([]Feed, error) {
	wellKnown := []string{
		"atom.xml",
		"feed.xml",
		"rss.xml",
		"index.xml",
		"atom.json",
		"feed.json",
		"rss.json",
		"index.json",
		"feed/",
		"rss/",
	}
	feeds := make([]Feed, 0)

	for _, suffix := range wellKnown {
		newTarget, err := url.JoinPath(baseURL, suffix)
		if err != nil {
			continue
		}
		feed, err := f.parseRSSUrl(ctx, newTarget)
		if err != nil {
			continue
		}
		if !isEmptyFeed(feed) {
			feed.Link = newTarget // this may be more accurate than the link parsed from the rss content
			feeds = append(feeds, feed)
		}
	}

	return feeds, nil
}

func (f *Finder) parseRSSUrl(ctx context.Context, target string) (Feed, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, target, nil)
	if err != nil {
		return Feed{}, err
	}
	resp, err := f.httpClient.Do(req)
	if err != nil {
		return Feed{}, err
	}
	defer resp.Body.Close()

	content, err := io.ReadAll(resp.Body)
	if err != nil {
		return Feed{}, err
	}
	return parseRSSContent(content)
}

func parseRSSContent(content []byte) (Feed, error) {
	parsed, err := gofeed.NewParser().Parse(bytes.NewReader(content))
	if err != nil || parsed == nil {
		return Feed{}, err
	}
	return Feed{
		// https://github.com/mmcdole/gofeed#default-mappings
		Title: parsed.Title,

		// set as default value, but the value parsed from rss are not always accurate.
		// it is better to use the url that gets the rss content.
		Link: parsed.FeedLink,
	}, nil
}
