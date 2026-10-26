package github

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
	"time"
)

// restCacheEntries bounds the conditional-response cache. A var so tests can lower it.
var restCacheEntries = 1000

type cachedREST struct {
	etag string
	body []byte
	next string // path of the rel="next" page from the Link header, "" if none
	used time.Time
}

func (client *Client) restGet(ctx context.Context, path string) (body []byte, next string, err error) {
	client.restMu.Lock()
	entry, ok := client.rest[path]
	client.restMu.Unlock()

	args := []string{"api", "-i"}
	if ok && entry.etag != "" {
		args = append(args, "-H", "If-None-Match: "+entry.etag)
	}
	args = append(args, path)
	out, execErr := exec.CommandContext(ctx, "gh", args...).Output()
	code, headers, body, parsed := parseREST(out)
	if !parsed {
		if execErr != nil {
			return nil, "", fmt.Errorf("gh api: %w", execErr)
		}
		return nil, "", fmt.Errorf("gh api: no status line")
	}
	if code == 304 {
		client.restMu.Lock()
		entry, ok = client.rest[path]
		if !ok {
			client.restMu.Unlock()
			return nil, "", fmt.Errorf("gh api %s: HTTP %d", path, code)
		}
		entry.used = time.Now()
		client.rest[path] = entry
		client.restMu.Unlock()
		return entry.body, entry.next, nil
	}
	if code < 200 || code >= 300 {
		return nil, "", fmt.Errorf("gh api %s: HTTP %d", path, code)
	}
	next, err = nextPagePath(headers["link"])
	if err != nil {
		return nil, "", err
	}
	client.restMu.Lock()
	client.rest[path] = cachedREST{etag: headers["etag"], body: body, next: next, used: time.Now()}
	client.evictREST()
	client.restMu.Unlock()
	return body, next, nil
}

func (client *Client) evictREST() {
	for len(client.rest) > restCacheEntries {
		var oldestKey string
		var oldest time.Time
		first := true
		for key, entry := range client.rest {
			if first || entry.used.Before(oldest) {
				first = false
				oldestKey = key
				oldest = entry.used
			}
		}
		delete(client.rest, oldestKey)
	}
}

func (client *Client) restJSON(ctx context.Context, dst any, path string) error {
	body, _, err := client.restGet(ctx, path)
	if err != nil {
		return err
	}
	if err = json.Unmarshal(body, dst); err != nil {
		return fmt.Errorf("decode GitHub response: %w", err)
	}
	return nil
}

func restAll[T any](ctx context.Context, client *Client, path string) ([]T, error) {
	var all []T
	for page := 0; ; page++ {
		if page == 100 {
			return nil, fmt.Errorf("gh api %s: exceeded 100 pages", path)
		}
		body, next, err := client.restGet(ctx, path)
		if err != nil {
			return nil, err
		}
		var items []T
		if err = json.Unmarshal(body, &items); err != nil {
			return nil, fmt.Errorf("decode GitHub response: %w", err)
		}
		all = append(all, items...)
		if next == "" {
			return all, nil
		}
		path = next
	}
}

func parseREST(out []byte) (code int, headers map[string]string, body []byte, ok bool) {
	line, rest, found := cutLine(out)
	if strings.TrimSpace(line) == "" {
		return 0, nil, nil, false
	}
	fields := strings.SplitN(line, " ", 3)
	if len(fields) < 2 || !strings.HasPrefix(fields[0], "HTTP/") {
		return 0, nil, nil, false
	}
	code, err := strconv.Atoi(fields[1])
	if err != nil {
		return 0, nil, nil, false
	}
	headers = map[string]string{}
	if !found {
		return code, headers, nil, true
	}
	for {
		line, rest, found = cutLine(rest)
		if line == "" {
			return code, headers, rest, true
		}
		if name, value, has := strings.Cut(line, ":"); has {
			headers[strings.ToLower(strings.TrimSpace(name))] = strings.TrimSpace(value)
		}
		if !found {
			return code, headers, nil, true
		}
	}
}

func cutLine(b []byte) (line string, rest []byte, found bool) {
	i := bytes.IndexByte(b, '\n')
	if i < 0 {
		return strings.TrimRight(string(b), "\r"), nil, false
	}
	return strings.TrimRight(string(b[:i]), "\r"), b[i+1:], true
}

// nextPagePath returns the rel="next" target as an api.github.com path plus query.
func nextPagePath(link string) (string, error) {
	for _, part := range strings.Split(link, ",") {
		target, params, _ := strings.Cut(part, ";")
		if !strings.Contains(params, `rel="next"`) {
			continue
		}
		path, ok := strings.CutPrefix(strings.Trim(strings.TrimSpace(target), "<>"), "https://api.github.com/")
		if !ok {
			return "", fmt.Errorf("gh api: next link is not on api.github.com")
		}
		return path, nil
	}
	return "", nil
}
