package feedfinder

import (
	"context"
	"fmt"
	"io"
	"regexp"
	"strings"
)

type serviceMatcher func(context.Context) ([]Feed, error)

func (f *Finder) tryService(ctx context.Context) ([]Feed, error) {
	matcher := []serviceMatcher{
		f.githubMatcher,
		f.redditMatcher,
		f.youtubeMatcher,
	}
	for _, fn := range matcher {
		feed, err := fn(ctx)
		if err != nil {
			continue
		}
		if len(feed) != 0 {
			return feed, nil
		}
	}
	return nil, nil
}

var githubGlobalFeed = []Feed{
	{Title: "global public timeline", Link: "https://github.com/timeline"},
	{Title: "global security advisories", Link: "https://github.com/security-advisories.atom"},
}

// https://docs.github.com/en/rest/activity/feeds?apiVersion=2022-11-28#get-feeds
func (s Finder) githubMatcher(ctx context.Context) ([]Feed, error) {
	if !strings.HasSuffix(s.target.Hostname(), "github.com") {
		return nil, nil
	}

	splited := strings.SplitN(s.target.Path, "/", 4) // split "/user/repo/" -> []string{"", "user", "repo", ""}
	splitedLen := len(splited)
	if splitedLen < 2 {
		return githubGlobalFeed, nil
	}

	username, reponame := "", ""
	if splitedLen >= 2 {
		username = splited[1]
	}
	if splitedLen >= 3 {
		reponame = splited[2]
	}

	if reponame != "" {
		re, err := regexp.Compile(`^[a-zA-Z0-9][a-zA-Z0-9-_\.]{0,98}[a-zA-Z0-9]$`) // todo need improve
		if err != nil {
			return nil, err
		}
		if !re.MatchString(reponame) {
			return nil, nil
		}
		return genGitHubRepoFeed(username + "/" + reponame), nil
	}

	if username != "" {
		re, err := regexp.Compile(`^[a-zA-Z0-9][-]?[a-zA-Z0-9]{0,38}$`)
		if err != nil {
			return nil, err
		}
		if !re.MatchString(username) {
			return nil, nil
		}
		return genGitHubUserFeed(username), nil
	}

	return nil, nil
}

func genGitHubUserFeed(username string) []Feed {
	return []Feed{{Title: username + " public timeline", Link: fmt.Sprintf("https://github.com/%s.atom", username)}}
}

func genGitHubRepoFeed(userRepo string) []Feed {
	return []Feed{
		{Title: fmt.Sprintf("%s commits", userRepo), Link: fmt.Sprintf("https://github.com/%s/commits.atom", userRepo)},
		{Title: fmt.Sprintf("%s releases", userRepo), Link: fmt.Sprintf("https://github.com/%s/releases.atom", userRepo)},
		{Title: fmt.Sprintf("%s tags", userRepo), Link: fmt.Sprintf("https://github.com/%s/tags.atom", userRepo)},
		{Title: fmt.Sprintf("%s wiki", userRepo), Link: fmt.Sprintf("https://github.com/%s/wiki.atom", userRepo)},
	}
}

var redditGlobalFeed = []Feed{
	{Title: "global", Link: "https://www.reddit.com/.rss"},
}

// https://www.reddit.com/wiki/rss/
func (s Finder) redditMatcher(ctx context.Context) ([]Feed, error) {
	if !strings.HasSuffix(s.target.Hostname(), "reddit.com") {
		return nil, nil
	}

	splited := strings.SplitN(s.target.Path, "/", 4)
	splitedLen := len(splited)
	if splitedLen < 2 {
		return redditGlobalFeed, nil
	}

	mode, param := splited[1], splited[2]
	switch mode {
	case "r":
		if splitedLen >= 4 && strings.HasPrefix(splited[3], "comments") {
			// "comments/{postID}/{title}"
			// "comments/{postID}/{title}/comment/{commentID}"
			return genRedditCommentFeed(s.target.String()), nil
		}
		return genRedditSubFeed(param), nil
	case "user":
		return genRedditUserFeed(param), nil
	case "domain":
		return genRedditDomainSubmissionFeed(param), nil
	default:
	}

	return nil, nil
}

func genRedditSubFeed(sub string) []Feed {
	return []Feed{
		{Title: fmt.Sprintf("/r/%s hot", sub), Link: fmt.Sprintf("https://reddit.com/r/%s/hot/.rss", sub)},
		{Title: fmt.Sprintf("/r/%s new", sub), Link: fmt.Sprintf("https://reddit.com/r/%s/new/.rss", sub)},
		{Title: fmt.Sprintf("/r/%s top", sub), Link: fmt.Sprintf("https://reddit.com/r/%s/top/.rss", sub)},
		{Title: fmt.Sprintf("/r/%s rising", sub), Link: fmt.Sprintf("https://reddit.com/r/%s/rising/.rss", sub)},
	}
}

func genRedditCommentFeed(fullURL string) []Feed {
	return []Feed{{Title: "post", Link: fullURL + ".rss"}}
}

func genRedditUserFeed(username string) []Feed {
	return []Feed{
		{Title: fmt.Sprintf("/u/%s overview new", username), Link: fmt.Sprintf("https://reddit.com/user/%s/.rss?sort=new", username)},
		{Title: fmt.Sprintf("/u/%s overview hot", username), Link: fmt.Sprintf("https://reddit.com/user/%s/.rss?sort=hot", username)},
		{Title: fmt.Sprintf("/u/%s overview top", username), Link: fmt.Sprintf("https://reddit.com/user/%s/.rss?sort=top", username)},
		{Title: fmt.Sprintf("/u/%s post new", username), Link: fmt.Sprintf("https://reddit.com/user/%s/submitted/.rss?sort=new", username)},
		{Title: fmt.Sprintf("/u/%s post hot", username), Link: fmt.Sprintf("https://reddit.com/user/%s/submitted/.rss?sort=hot", username)},
		{Title: fmt.Sprintf("/u/%s post top", username), Link: fmt.Sprintf("https://reddit.com/user/%s/submitted/.rss?sort=top", username)},
		{Title: fmt.Sprintf("/u/%s comments new", username), Link: fmt.Sprintf("https://reddit.com/user/%s/comments/.rss?sort=new", username)},
		{Title: fmt.Sprintf("/u/%s comments hot", username), Link: fmt.Sprintf("https://reddit.com/user/%s/comments/.rss?sort=hot", username)},
		{Title: fmt.Sprintf("/u/%s comments top", username), Link: fmt.Sprintf("https://reddit.com/user/%s/comments/.rss?sort=top", username)},
		{Title: fmt.Sprintf("/u/%s awards received (legacy)", username), Link: fmt.Sprintf("https://old.reddit.com/user/%s/gilded/.rss", username)},
	}
}

func genRedditDomainSubmissionFeed(domain string) []Feed {
	return []Feed{{Title: "/domain/" + domain, Link: fmt.Sprintf("https://reddit.com/domain/%s/.rss", domain)}}
}

func (s Finder) youtubeMatcher(ctx context.Context) ([]Feed, error) {
	if !strings.HasSuffix(s.target.Hostname(), "youtube.com") && !strings.HasSuffix(s.target.Hostname(), "youtu.be") {
		return nil, nil
	}

	resp, err := s.httpClient.Get(s.target.String())
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	content, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if strings.HasPrefix(s.target.Path, "/@") {
		re, err := regexp.Compile(`{"key":"browse_id","value":"(.+?)"}`)
		if err != nil {
			return nil, err
		}
		match := re.FindStringSubmatch(string(content))
		if len(match) < 2 {
			return nil, nil
		}
		id := match[1]
		if id == "" {
			return nil, nil
		}
		return []Feed{{Title: "Channel", Link: "https://www.youtube.com/feeds/videos.xml?channel_id=" + id}}, nil
	} else if strings.HasPrefix(s.target.Path, "/playlist") {
		id := s.target.Query().Get("list")
		if id == "" {
			return nil, nil
		}
		return []Feed{{Title: "Playlist", Link: "https://www.youtube.com/feeds/videos.xml?playlist_id=" + id}}, nil
	}

	return nil, nil
}
