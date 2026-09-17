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
	// tokenEnvVar pins the session's token, taking precedence over chainctl's
	// cache. See TokenFromEnv.
	tokenEnvVar = "CHAINGUARD_TOKEN"
)

type Client struct {
	v2        v2beta1.Clients
	platform  platform.Clients // v1 — SBOM / manifest metadata only
	libraries librariesv2.Clients
	libConn   *grpc.ClientConn // owns libraries connection
	token     string
	subject   string
	identity  string
	email     string
}

// Subject returns the human behind the session: the JWT sub claim, or the actor
// (act.sub) when the token was obtained by assuming an identity.
func (c *Client) Subject() string { return c.subject }

// Identity returns the identity the session is acting as — the token's own sub
// claim. It differs from Subject only while an identity is being assumed.
func (c *Client) Identity() string { return c.identity }

// Assumed reports whether this session is acting as an assumed identity rather
// than as the logged-in human directly.
func (c *Client) Assumed() bool { return c.identity != "" && c.identity != c.subject }

// Email returns the authenticated user's email if present in the token.
func (c *Client) Email() string { return c.email }

// TokenFromEnv reports whether the session's token came from CHAINGUARD_TOKEN
// rather than chainctl's cache. Switching identity re-runs chainctl login, which
// has no effect while the environment pins the token.
func TokenFromEnv() bool { return os.Getenv(tokenEnvVar) != "" }

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

// LoginCommand builds the chainctl login command. An identityUID assumes that
// identity; empty logs in as the caller themselves, which is also how you stop
// assuming one.
//
// The command is interactive — it may open a browser — so a caller that has the
// terminal must hand it back first (tea.ExecProcess does this).
func LoginCommand(identityUID string) (*exec.Cmd, error) {
	if _, err := exec.LookPath("chainctl"); err != nil {
		return nil, fmt.Errorf("%w: chainctl not found in PATH (install the Chainguard CLI or set %s)", ErrNotLoggedIn, tokenEnvVar)
	}
	args := []string{"auth", "login"}
	if id := strings.TrimSpace(identityUID); id != "" {
		args = append(args, "--identity="+id)
	}
	return exec.Command("chainctl", args...), nil
}

// Login runs chainctl auth login interactively (inheriting the terminal),
// then returns a ready Client using the freshly issued token.
// Call this only before the TUI has taken over the terminal.
func Login() (*Client, error) {
	cmd, err := LoginCommand("")
	if err != nil {
		return nil, err
	}
	fmt.Fprintln(os.Stderr, "Starting chainctl auth login...")
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

	sub, id, email := parseToken(token)
	return &Client{
		v2:        v2,
		platform:  p,
		libraries: libs,
		libConn:   libConn,
		token:     token,
		subject:   sub,
		identity:  id,
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

// parseToken extracts the subject, the assumed identity and the email from a
// JWT without validating its signature.
//
// identity is the token's own sub claim — who the session acts as. subject is
// the human behind it, which is the same thing unless an identity is being
// assumed, in which case act.sub names the actor who assumed it.
func parseToken(token string) (subject, identity, email string) {
	if _, sub, err := cgauth.ExtractIssuerAndSubject(token); err == nil {
		identity, subject = sub, sub
	}
	if em, _, err := cgauth.ExtractEmail(token); err == nil {
		email = em
	}
	// act.sub is the human actor when the token was obtained by assuming an
	// identity; the identity itself stays in sub.
	if actor, err := cgauth.ExtractActor(token); err == nil && actor.Subject != "" {
		subject = actor.Subject
	}
	return
}

// ErrNotLoggedIn is returned when no valid token can be found.
var ErrNotLoggedIn = fmt.Errorf("not logged in to Chainguard")
