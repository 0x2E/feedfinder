package feedfinder

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"sync"
)

type Feed struct {
	Title string `json:"title"`
	Link  string `json:"link"`
}

type Finder struct {
	target     *url.URL
	httpClient *http.Client
}

type Options struct {
	// ReqestProxy is the proxy url for HTTP client
	ReqestProxy *string
}

func Find(ctx context.Context, target string, options *Options) ([]Feed, error) {
	u, err := url.Parse(target)
	if err != nil {
		return nil, err
	}

	clientTransportOps := []transportOptionFunc{}
	if options != nil {
		if options.ReqestProxy != nil && *options.ReqestProxy != "" {
			proxyURL, err := url.Parse(*options.ReqestProxy)
			if err != nil {
				return nil, err
			}
			clientTransportOps = append(clientTransportOps, func(transport *http.Transport) {
				transport.Proxy = http.ProxyURL(proxyURL)
			})
		}
	}

	finder := Finder{
		target:     u,
		httpClient: newClient(clientTransportOps...),
	}
	return finder.Run(context.Background())
}

func (f *Finder) Run(ctx context.Context) ([]Feed, error) {
	// find in third-party service
	fromService, err := f.tryService(ctx)
	if err != nil {
		return nil, err
	}
	if len(fromService) != 0 {
		return fromService, nil
	}

	feedMap := make(map[string]Feed)
	mu := sync.Mutex{}
	wg := sync.WaitGroup{}

	wg.Add(1)
	go func() {
		defer wg.Done()

		// sniff in HTML
		data, err := f.tryPageSource(ctx)
		if err != nil {
			slog.Debug("failed to sniff in HTML", "err", err)
		}

		mu.Lock()
		for _, f := range data {
			feedMap[f.Link] = f
		}
		mu.Unlock()
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()

		// sniff well-knowns under this url
		data, err := f.tryWellKnown(ctx, fmt.Sprintf("%s://%s%s", f.target.Scheme, f.target.Host, f.target.Path))
		if err != nil {
			slog.Debug("failed to sniff well-knowns", "error", err)
		}
		if len(data) == 0 {
			// sniff well-knowns under root path
			data, err = f.tryWellKnown(ctx, fmt.Sprintf("%s://%s", f.target.Scheme, f.target.Host))
			if err != nil {
				slog.Debug("failed to sniff well-knowns under the root path", "error", err)
			}
		}

		mu.Lock()
		for _, f := range data {
			feedMap[f.Link] = f
		}
		mu.Unlock()
	}()

	wg.Wait()
	res := make([]Feed, 0, len(feedMap))
	for _, f := range feedMap {
		res = append(res, f)
	}
	return res, nil
}

func isEmptyFeedLink(feed Feed) bool {
	return feed == Feed{}
}

func absURL(base, link string) string {
	if link == "" {
		return base
	}

	linkURL, err := url.Parse(link)
	if err != nil {
		return link
	}
	if linkURL.IsAbs() {
		return link
	}

	baseURL, err := url.Parse(base)
	if err != nil {
		return link
	}
	return baseURL.ResolveReference(linkURL).String()
}
