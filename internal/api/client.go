package api

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"

	cgauth "chainguard.dev/sdk/auth"
	"chainguard.dev/sdk/proto/platform"
)

const apiBase = "https://console-api.enforce.dev"

type Client struct {
	platform platform.Clients
	token    string
	subject  string
	email    string
}

// Subject returns the authenticated identity's UIDP (from the JWT sub claim).
func (c *Client) Subject() string { return c.subject }

// Email returns the authenticated user's email if present in the token.
func (c *Client) Email() string { return c.email }

// NewClient resolves a token from the environment or chainctl's token cache.
// Returns an error (with ErrNotLoggedIn) if no cached token exists.
func NewClient() (*Client, error) {
	token, err := cachedToken()
	if err != nil {
		return nil, err
	}
	return newClient(token)
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
	return newClient(token)
}

func newClient(token string) (*Client, error) {
	ctx := context.Background()
	cred := cgauth.NewFromToken(ctx, token, false)
	p, err := platform.NewPlatformClients(ctx, apiBase, cred)
	if err != nil {
		return nil, fmt.Errorf("create platform clients: %w", err)
	}
	sub, email := parseToken(token)
	return &Client{
		platform: p,
		token:    token,
		subject:  sub,
		email:    email,
	}, nil
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

// parseToken extracts subject and email from a JWT without validating its signature.
// Handles act.sub (impersonation) by preferring the actor subject when present.
func parseToken(token string) (subject, email string) {
	_, sub, err := cgauth.ExtractIssuerAndSubject(token)
	if err == nil {
		subject = sub
	}
	em, _, err := cgauth.ExtractEmail(token)
	if err == nil {
		email = em
	}
	// act.sub is the human actor when the token was obtained via impersonation/delegation.
	if actor, err := cgauth.ExtractActor(token); err == nil && actor.Subject != "" {
		subject = actor.Subject
	}
	return
}

// ErrNotLoggedIn is returned when no valid token can be found.
var ErrNotLoggedIn = fmt.Errorf("not logged in (run chainctl auth login, or set CHAINGUARD_TOKEN)")
