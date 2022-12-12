package secretscan

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/pkg/errors"
)

const timeout = 4 * time.Second
const maxTokensPerRequest = 100

// SecretScan Client is grafana's client for checking leaked keys.
// Don't use this client directly,
// use the secretscan Service which handles token collection and checking instead.
type client struct {
	httpClient *http.Client
	version    string
	baseURL    string
}

type secretscanRequest struct {
	KeyHashes []string `json:"hashes"`
}

type Token struct {
	Type       string `json:"type"`
	URL        string `json:"url"`
	Hash       string `json:"hash"`
	ReportedAt string `json:"reported_at"` //nolint
}

var ErrInvalidStatusCode = errors.New("invalid status code")

func newClient(url, version string) *client {
	return &client{
		version: version,
		baseURL: url,
		httpClient: &http.Client{
			Timeout:       timeout,
			Transport:     nil,
			CheckRedirect: nil,
			Jar:           nil,
		},
	}
}

// checkTokens checks if any leaked tokens exist.
// Returns list of leaked tokens.
func (c *client) CheckTokens(ctx context.Context, keyHashes []string) ([]Token, error) {
	// decode response body
	tokens := make([]Token, 0, len(keyHashes))

	// batch requests to secretscan server
	err := batch(len(keyHashes), maxTokensPerRequest, func(start, end int) error {
		bTokens, err := c.checkTokens(ctx, keyHashes[start:end])
		if err != nil {
			return err
		}

		tokens = append(tokens, bTokens...)
		return nil
	})

	if err != nil {
		return nil, err
	}

	return tokens, nil
}

func (c *client) checkTokens(ctx context.Context, keyHashes []string) ([]Token, error) {
	// create request body
	values := secretscanRequest{KeyHashes: keyHashes}

	jsonValue, err := json.Marshal(values)
	if err != nil {
		return nil, errors.Wrap(err, "failed to make http request")
	}

	// Build URL
	url := fmt.Sprintf("%s/tokens", c.baseURL)
	// Create request for secretscan server
	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		url, bytes.NewReader(jsonValue))
	if err != nil {
		return nil, errors.Wrap(err, "failed to make http request")
	}

	// Set headers
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "grafana-secretscan-client/"+c.version)

	// make http POST request to check for leaked tokens.
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, errors.Wrap(err, "failed to do http request")
	}

	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("%w. status code: %s", ErrInvalidStatusCode, resp.Status)
	}

	// decode response body
	var tokens []Token
	if err := json.NewDecoder(resp.Body).Decode(&tokens); err != nil {
		return nil, errors.Wrap(err, "failed to decode response body")
	}

	return tokens, nil
}

func batch(count, size int, eachFn func(start, end int) error) error {
	for i := 0; i < count; {
		end := i + size
		if end > count {
			end = count
		}

		if err := eachFn(i, end); err != nil {
			return err
		}

		i = end
	}

	return nil
}
