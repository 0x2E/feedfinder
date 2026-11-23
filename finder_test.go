package feedfinder

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/assert"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

func testResponse(req *http.Request, status int, contentType, body string) *http.Response {
	return &http.Response{
		StatusCode: status,
		Header:     http.Header{"Content-Type": []string{contentType}},
		Body:       io.NopCloser(strings.NewReader(body)),
		Request:    req,
	}
}

func TestFormatLinkToAbs(t *testing.T) {
	type item struct {
		base string
		link string
		want string
	}
	table := []item{
		{base: "https://x.xx", link: "https://1.xx", want: "https://1.xx"},
		{base: "https://x.xx", link: "", want: "https://x.xx"},
		{base: "https://x.xx/1/", link: "/x/index.xml", want: "https://x.xx/x/index.xml"},
		{base: "https://x.xx/1/", link: "x/index.xml", want: "https://x.xx/1/x/index.xml"},
		{base: "https://x.xx/1", link: "index.xml", want: "https://x.xx/index.xml"},
	}

	for _, tt := range table {
		res := absURL(tt.base, tt.link)
		assert.Equal(t, tt.want, res)
	}
}

func TestFindUsesInjectedHTTPClient(t *testing.T) {
	var serverHits atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		serverHits.Add(1)
		http.NotFound(w, r)
	}))
	defer server.Close()

	pageURL := server.URL + "/page"
	html := `<html><head><title>page</title></head><body><a href="` + server.URL + `/rss-link.xml">RSS</a></body></html>`
	rss := `<?xml version="1.0" encoding="utf-8"?><rss version="2.0"><channel><title>feed</title><link>https://example.com/</link></channel></rss>`

	requests := make(map[string]int)
	var mu sync.Mutex
	client := &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		mu.Lock()
		requests[req.URL.Path]++
		mu.Unlock()

		switch {
		case req.URL.Path == "/page":
			return testResponse(req, http.StatusOK, "text/html", html), nil
		case strings.HasPrefix(req.URL.Path, "/page/"):
			// Make the well-known probes under the page path fail so the
			// finder falls back to probing them under the root path.
			return testResponse(req, http.StatusNotFound, "text/plain", ""), nil
		default:
			return testResponse(req, http.StatusOK, "application/rss+xml", rss), nil
		}
	})}

	feeds, err := Find(context.Background(), pageURL, &Options{HTTPClient: client})
	assert.NoError(t, err)
	assert.NotEmpty(t, feeds)
	assert.Zero(t, serverHits.Load(), "outbound requests bypassed the injected client")

	mu.Lock()
	defer mu.Unlock()
	assert.NotZero(t, requests["/page"], "page fetch did not use the injected client")
	assert.NotZero(t, requests["/rss-link.xml"], "HTML link probe did not use the injected client")
	assert.NotZero(t, requests["/page/atom.xml"], "path well-known probe did not use the injected client")
	assert.NotZero(t, requests["/atom.xml"], "root well-known probe did not use the injected client")
}

func TestFindUsesInjectedHTTPClientForService(t *testing.T) {
	// Port 1 is unreachable, so the test cannot accidentally hit YouTube if
	// the injected client is not used.
	target := "http://www.youtube.com:1/@example"
	requested := make([]string, 0, 1)
	client := &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		requested = append(requested, req.URL.String())
		return testResponse(req, http.StatusOK, "text/html", `{"key":"browse_id","value":"UC123"}`), nil
	})}

	feeds, err := Find(context.Background(), target, &Options{HTTPClient: client})
	assert.NoError(t, err)
	assert.Equal(t, []string{target}, requested)
	assert.Equal(t, []Feed{{Title: "Channel", Link: "https://www.youtube.com/feeds/videos.xml?channel_id=UC123"}}, feeds)
}

func TestFindRejectsHTTPClientWithRequestProxy(t *testing.T) {
	proxy := "http://127.0.0.1:8080"
	_, err := Find(context.Background(), "https://example.com", &Options{
		HTTPClient:   http.DefaultClient,
		RequestProxy: &proxy,
	})
	assert.ErrorContains(t, err, "HTTPClient and RequestProxy cannot both be set")
}

func TestFindAllowsHTTPClientWithEmptyRequestProxy(t *testing.T) {
	empty := ""
	client := &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		return testResponse(req, http.StatusNotFound, "text/plain", ""), nil
	})}

	_, err := Find(context.Background(), "https://example.com", &Options{
		HTTPClient:   client,
		RequestProxy: &empty,
	})
	assert.NoError(t, err)
}

func TestFindFollowsRedirectsWithInjectedHTTPClient(t *testing.T) {
	var mu sync.Mutex
	requested := make(map[string]int)
	client := &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		mu.Lock()
		requested[req.URL.Host+req.URL.Path]++
		mu.Unlock()

		if req.URL.Host == "start.invalid" && req.URL.Path == "/page" {
			resp := testResponse(req, http.StatusFound, "text/plain", "")
			resp.Header.Set("Location", "http://redirected.invalid/target")
			return resp, nil
		}
		return testResponse(req, http.StatusNotFound, "text/plain", ""), nil
	})}

	_, err := Find(context.Background(), "http://start.invalid/page", &Options{HTTPClient: client})
	assert.NoError(t, err)

	mu.Lock()
	defer mu.Unlock()
	assert.NotZero(t, requested["redirected.invalid/target"], "redirect did not use the injected client")
}

func TestFindUsesRequestProxyWithDefaultClient(t *testing.T) {
	rss := `<?xml version="1.0" encoding="utf-8"?><rss version="2.0"><channel><title>feed</title></channel></rss>`
	var proxyHits atomic.Int32
	proxy := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		proxyHits.Add(1)
		if r.URL.Host == "feeds.invalid" && r.URL.Path == "/atom.xml" {
			w.Header().Set("Content-Type", "application/rss+xml")
			_, _ = io.WriteString(w, rss)
			return
		}
		http.NotFound(w, r)
	}))
	defer proxy.Close()

	proxyURL := proxy.URL
	feeds, err := Find(context.Background(), "http://feeds.invalid/", &Options{RequestProxy: &proxyURL})
	assert.NoError(t, err)
	assert.NotZero(t, proxyHits.Load(), "requests did not go through RequestProxy")
	assert.Equal(t, []Feed{{Title: "feed", Link: "http://feeds.invalid/atom.xml"}}, feeds)
}
