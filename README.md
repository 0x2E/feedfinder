# feedfinder

feedfinder is a library for finding RSS and Atom feeds on a website.

## Usage

1. Install the package:

```shell
go get github.com/0x2E/feedfinder
```

2. Import the package:

```go

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
