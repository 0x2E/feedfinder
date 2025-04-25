# feedfinder

feedfinder is a library for finding RSS and Atom feeds on a website.

## Usage

1. Install the package:

```shell
go get github.com/0x2E/feedfinder
```

2. Import the package:

```go
package main

import (
	"context"
	"fmt"
	"net/url"

	"github.com/0x2E/feedfinder"
)

func main() {
	link := "https://github.com/golang/go"

	feeds, err := feedfinder.Find(context.Background(), link, nil)
	if err != nil {
		panic(err)
	}

	for _, feed := range feeds {
		fmt.Printf("title: %s\tlink: %s\n", feed.Title, feed.Link)
	}
}

// Output:
// title: golang/go commits        link: https://github.com/golang/go/commits.atom
// title: golang/go releases       link: https://github.com/golang/go/releases.atom
// title: golang/go tags   link: https://github.com/golang/go/tags.atom
// title: golang/go wiki   link: https://github.com/golang/go/wiki.atom
```

## How it works

It tries to find feeds in the following ways:

**Parsing HTML**:

- `<link>` with type `application/rss+xml`, `application/atom+xml`, `application/json`, `application/feed+json`
- `<a>` containing the word `rss`

**Well-known paths**:

- `atom.xml`, `feed.xml`, `rss.xml`, `index.xml`
- `atom.json`, `feed.json`, `rss.json`, `index.json`
- `feed/`, `rss/`

**Third party services**:

- GitHub: [official rules](https://docs.github.com/en/rest/activity/feeds?apiVersion=2022-11-28)
- Reddit: [official wiki](https://www.reddit.com/wiki/rss/)
- YouTube: [ref](https://authory.com/blog/create-a-youtube-rss-feed-with-vastly-increased-limits)

## Credits

- Parsing feed with [gofeed](https://github.com/mmcdole/gofeed)
