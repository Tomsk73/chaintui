package api

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"
)

const apiBase = "https://console-api.enforce.dev"

type Client struct {
	http  *http.Client
	mu    sync.Mutex
	token string
}

// NewClient resolves a token from the environment or chainctl's token cache.
// Returns an error (with ErrNotLoggedIn) if no cached token exists.
func NewClient() (*Client, error) {
	token, err := cachedToken()
	if err != nil {
		return nil, err
	}
	return newClient(token), nil
}

// Login runs chainctl auth login interactively (inheriting the terminal),
// then returns a ready Client using the freshly issued token.
// Call this only before the TUI has taken over the terminal.
func Login() (*Client, error) {
	cmd := exec.Command("chainctl", "auth", "login")
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("chainctl auth login: %w", err)
	}
	token, err := cachedToken()
	if err != nil {
		return nil, fmt.Errorf("fetch token after login: %w", err)
	}
	return newClient(token), nil
}

func newClient(token string) *Client {
	return &Client{
		http:  &http.Client{Timeout: 30 * time.Second},
		token: token,
	}
}

// cachedToken returns a token from the environment or chainctl's cache.
func cachedToken() (string, error) {
	if t := os.Getenv("CHAINGUARD_TOKEN"); t != "" {
		return t, nil
	}
	out, err := exec.Command("chainctl", "auth", "token").Output()
	if err != nil {
		return "", ErrNotLoggedIn
	}
	t := strings.TrimSpace(string(out))
	if t == "" {
		return "", ErrNotLoggedIn
	}
	return t, nil
}

// ErrNotLoggedIn is returned when no valid token can be found.
var ErrNotLoggedIn = fmt.Errorf("not logged in (run chainctl auth login, or set CHAINGUARD_TOKEN)")

func (c *Client) get(path string, params url.Values, out any) error {
	return c.doGet(path, params, out, true)
}

func (c *Client) doGet(path string, params url.Values, out any, allowRefresh bool) error {
	u, err := url.Parse(apiBase + path)
	if err != nil {
		return err
	}
	if len(params) > 0 {
		u.RawQuery = params.Encode()
	}

	req, err := http.NewRequest(http.MethodGet, u.String(), nil)
	if err != nil {
		return err
	}

	c.mu.Lock()
	req.Header.Set("Authorization", "Bearer "+c.token)
	c.mu.Unlock()
	req.Header.Set("Accept", "application/json")

	resp, err := c.http.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}

	if resp.StatusCode == http.StatusUnauthorized && allowRefresh {
		// Token may have expired — try refreshing once from the cache.
		if newToken, err := cachedToken(); err == nil {
			c.mu.Lock()
			c.token = newToken
			c.mu.Unlock()
			return c.doGet(path, params, out, false)
		}
		return ErrNotLoggedIn
	}

	if resp.StatusCode != http.StatusOK {
		var errResp struct {
			Message string `json:"message"`
		}
		if json.Unmarshal(body, &errResp) == nil && errResp.Message != "" {
			return fmt.Errorf("API %d: %s", resp.StatusCode, errResp.Message)
		}
		return fmt.Errorf("API %d", resp.StatusCode)
	}

	return json.Unmarshal(body, out)
}
