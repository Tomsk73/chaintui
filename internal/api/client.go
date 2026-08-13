package api

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"strings"

	delegate "chainguard.dev/go-grpc-kit/pkg/options"
	cgauth "chainguard.dev/sdk/auth"
	v2beta1 "chainguard.dev/sdk/proto/chainguard/platform/clients/v2beta1"
	librariesv2 "chainguard.dev/sdk/proto/chainguard/platform/libraries/v2beta1"
	"chainguard.dev/sdk/proto/platform"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
)

const (
	apiBase   = "https://console-api.enforce.dev"
	userAgent = "chaintui"
)

type Client struct {
	v2        v2beta1.Clients
	platform  platform.Clients // v1 — SBOM / manifest metadata only
	libraries librariesv2.Clients
	libConn   *grpc.ClientConn // owns libraries connection
	token     string
	subject   string
	email     string
}

// Subject returns the authenticated identity's UIDP (from the JWT sub claim).
func (c *Client) Subject() string { return c.subject }

// Email returns the authenticated user's email if present in the token.
func (c *Client) Email() string { return c.email }

// Close releases API connections. Safe to call multiple times.
func (c *Client) Close() error {
	var first error
	if c.v2 != nil {
		if err := c.v2.Close(); err != nil && first == nil {
			first = err
		}
	}
	if c.platform != nil {
		if err := c.platform.Close(); err != nil && first == nil {
			first = err
		}
	}
	if c.libConn != nil {
		if err := c.libConn.Close(); err != nil && first == nil {
			first = err
		}
		c.libConn = nil
	}
	return first
}

// NewClient resolves a token from the environment or chainctl's token cache.
// Returns an error matching ErrNotLoggedIn if no cached token exists.
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
	if _, err := exec.LookPath("chainctl"); err != nil {
		return nil, fmt.Errorf("%w: chainctl not found in PATH (install the Chainguard CLI or set CHAINGUARD_TOKEN)", ErrNotLoggedIn)
	}
	fmt.Fprintln(os.Stderr, "Starting chainctl auth login...")
	cmd := exec.Command("chainctl", "auth", "login")
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("chainctl auth login failed: %w", err)
	}
	token, err := cachedToken()
	if err != nil {
		return nil, fmt.Errorf("no token available after login: %w", err)
	}
	return newClient(token)
}

// IsNotLoggedIn reports whether err indicates missing Chainguard credentials.
func IsNotLoggedIn(err error) bool {
	return errors.Is(err, ErrNotLoggedIn)
}

func newClient(token string) (*Client, error) {
	ctx := context.Background()
	cred := cgauth.NewFromToken(ctx, token, false)

	v2, err := v2beta1.NewClients(ctx, apiBase, userAgent, cred)
	if err != nil {
		return nil, fmt.Errorf("create v2beta1 clients: %w", err)
	}

	p, err := platform.NewPlatformClients(ctx, apiBase, cred)
	if err != nil {
		_ = v2.Close()
		return nil, fmt.Errorf("create platform clients: %w", err)
	}

	libConn, libs, err := dialLibraries(cred)
	if err != nil {
		_ = v2.Close()
		_ = p.Close()
		return nil, fmt.Errorf("create libraries clients: %w", err)
	}

	sub, email := parseToken(token)
	return &Client{
		v2:        v2,
		platform:  p,
		libraries: libs,
		libConn:   libConn,
		token:     token,
		subject:   sub,
		email:     email,
	}, nil
}

func dialLibraries(cred credentials.PerRPCCredentials) (*grpc.ClientConn, librariesv2.Clients, error) {
	uri, err := url.Parse(apiBase)
	if err != nil {
		return nil, nil, err
	}
	target, opts := delegate.GRPCOptions(*uri)
	opts = append(opts, grpc.WithPerRPCCredentials(cred), grpc.WithUserAgent(userAgent))
	conn, err := grpc.NewClient(target, opts...)
	if err != nil {
		return nil, nil, err
	}
	return conn, librariesv2.NewClientsFromConnection(conn), nil
}

// cachedToken returns a token from the environment or chainctl's cache.
func cachedToken() (string, error) {
	if t := os.Getenv("CHAINGUARD_TOKEN"); t != "" {
		return t, nil
	}
	if _, err := exec.LookPath("chainctl"); err != nil {
		return "", fmt.Errorf("%w: chainctl not found in PATH (install the Chainguard CLI or set CHAINGUARD_TOKEN)", ErrNotLoggedIn)
	}
	out, err := exec.Command("chainctl", "auth", "token").Output()
	if err != nil {
		return "", fmt.Errorf("%w: no chainctl token cache (run chainctl auth login)", ErrNotLoggedIn)
	}
	t := strings.TrimSpace(string(out))
	if t == "" {
		return "", fmt.Errorf("%w: empty chainctl token (run chainctl auth login)", ErrNotLoggedIn)
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
var ErrNotLoggedIn = fmt.Errorf("not logged in to Chainguard")
