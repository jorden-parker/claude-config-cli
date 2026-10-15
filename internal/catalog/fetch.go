package catalog

import (
	"context"
	"fmt"
	"io"
	"net/http"
)

// DefaultBase is where the docs live.
const DefaultBase = "https://code.claude.com/docs/en/"

// maxPage caps a downloaded page.
const maxPage = 16 << 20

// Fetch downloads the three pages from base using client.
func Fetch(ctx context.Context, client *http.Client, base string) (Inputs, error) {
	var in Inputs
	for i, dst := range []*[]byte{&in.Ref, &in.Env, &in.Tools} {
		b, err := get(ctx, client, base+Pages[i])
		if err != nil {
			return in, err
		}
		*dst = b
	}
	return in, nil
}

func get(ctx context.Context, client *http.Client, url string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "ccfg-sync")
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetch %s: %w", url, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("fetch %s: HTTP %s", url, resp.Status)
	}
	b, err := io.ReadAll(io.LimitReader(resp.Body, maxPage+1))
	if err != nil {
		return nil, fmt.Errorf("fetch %s: %w", url, err)
	}
	if len(b) > maxPage {
		return nil, fmt.Errorf("fetch %s: page larger than %d bytes", url, maxPage)
	}
	return b, nil
}
