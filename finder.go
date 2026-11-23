package feedfinder

import (
	"context"
	"errors"
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
	// RequestProxy is the proxy url for the default HTTP client.
	// It must be empty when HTTPClient is set.
	RequestProxy *string

	// HTTPClient is used for all outbound requests made by Find, including
	// redirects and URLs discovered while probing. It is used as is, so its
	// Timeout and CheckRedirect apply. When set, RequestProxy must be empty
	// because the proxy belongs on the client.
	HTTPClient *http.Client
}

func Find(ctx context.Context, target string, options *Options) ([]Feed, error) {
	u, err := url.Parse(target)
	if err != nil {
		return nil, err
	}

	var httpClient *http.Client
	clientTransportOps := []transportOptionFunc{}
	if options != nil {
		useProxy := options.RequestProxy != nil && *options.RequestProxy != ""
		if options.HTTPClient != nil {
			if useProxy {
				return nil, errors.New("HTTPClient and RequestProxy cannot both be set")
			}
			httpClient = options.HTTPClient
		} else if useProxy {
			proxyURL, err := url.Parse(*options.RequestProxy)
			if err != nil {
				return nil, err
			}
			clientTransportOps = append(clientTransportOps, func(transport *http.Transport) {
				transport.Proxy = http.ProxyURL(proxyURL)
			})
		}
	}
	if httpClient == nil {
		httpClient = newClient(clientTransportOps...)
	}

	finder := Finder{
		target:     u,
		httpClient: httpClient,
	}
	return finder.Run(ctx)
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

// isEmptyFeed reports whether both Title and Link are zero values.
func isEmptyFeed(feed Feed) bool {
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
