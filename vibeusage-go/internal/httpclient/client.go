package httpclient

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// Client wraps net/http.Client with convenience methods for JSON APIs.
type Client struct {
	http *http.Client
}

// Response wraps the status code, body bytes, and optional JSON decode error
// from a completed HTTP request. The underlying http.Response body is already
// closed; callers read from Body instead.
type Response struct {
	StatusCode int
	Body       []byte
	JSONErr    error
}

// New creates a Client with a 30-second timeout.
func New() *Client {
	return &Client{http: &http.Client{Timeout: 30 * time.Second}}
}

// NewWithTimeout creates a Client with the given timeout.
func NewWithTimeout(timeout time.Duration) *Client {
	return &Client{http: &http.Client{Timeout: timeout}}
}

// NewFromConfig creates a Client using the config timeout (in seconds).
// Falls back to 30s if the value is zero or negative.
func NewFromConfig(timeoutSeconds float64) *Client {
	if timeoutSeconds <= 0 {
		return New()
	}
	return NewWithTimeout(time.Duration(timeoutSeconds * float64(time.Second)))
}

// RequestOption configures an http.Request before it is sent.
type RequestOption func(*http.Request)

// DoCtx sends an HTTP request with the given context, method and URL, applies
// options, reads the full body, and returns a Response. The context controls
// cancellation and deadlines for the request. The body parameter is optional. A
// non-nil error indicates a network-level failure (DNS, connect, timeout) or
// context cancellation; HTTP error status codes are returned in
// Response.StatusCode.
func (c *Client) DoCtx(ctx context.Context, method, rawURL string, body io.Reader, opts ...RequestOption) (*Response, error) {
	req, err := http.NewRequestWithContext(ctx, method, rawURL, body)
	if err != nil {
		return nil, err
	}
	for _, opt := range opts {
		opt(req)
	}

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	return &Response{StatusCode: resp.StatusCode, Body: respBody}, nil
}

// Do sends an HTTP request with the given method and URL, applies options, reads
// the full body, and returns a Response. The body parameter is optional. A
// non-nil error indicates a network-level failure (DNS, connect, timeout); HTTP
// error status codes are returned in Response.StatusCode.
func (c *Client) Do(method, rawURL string, body io.Reader, opts ...RequestOption) (*Response, error) {
	return c.DoCtx(context.Background(), method, rawURL, body, opts...)
}

// GetJSONCtx sends a context-aware GET request and decodes the response body as
// JSON into out. If out is nil, the body is still read and captured but not
// decoded. HTTP error status codes are returned in Response; JSON decode errors
// are captured in Response.JSONErr rather than returned as the function error.
func (c *Client) GetJSONCtx(ctx context.Context, rawURL string, out any, opts ...RequestOption) (*Response, error) {
	resp, err := c.DoCtx(ctx, http.MethodGet, rawURL, nil, opts...)
	if err != nil {
		return nil, err
	}
	if out != nil {
		resp.JSONErr = json.Unmarshal(resp.Body, out)
	}
	return resp, nil
}

// GetJSON sends a GET request and decodes the response body as JSON into out.
// If out is nil, the body is still read and captured but not decoded. HTTP error
// status codes are returned in Response; JSON decode errors are captured in
// Response.JSONErr rather than returned as the function error.
func (c *Client) GetJSON(rawURL string, out any, opts ...RequestOption) (*Response, error) {
	return c.GetJSONCtx(context.Background(), rawURL, out, opts...)
}

// PostJSONCtx sends a context-aware POST request with a JSON-encoded body and
// decodes the response as JSON into out. If body is nil the request has no body.
// If out is nil the response body is not decoded. Content-Type is set to
// application/json automatically.
func (c *Client) PostJSONCtx(ctx context.Context, rawURL string, body any, out any, opts ...RequestOption) (*Response, error) {
	var reader io.Reader
	if body != nil {
		encoded, err := json.Marshal(body)
		if err != nil {
			return nil, err
		}
		reader = bytes.NewReader(encoded)
	}
	allOpts := append([]RequestOption{WithHeader("Content-Type", "application/json")}, opts...)
	resp, err := c.DoCtx(ctx, http.MethodPost, rawURL, reader, allOpts...)
	if err != nil {
		return nil, err
	}
	if out != nil {
		resp.JSONErr = json.Unmarshal(resp.Body, out)
	}
	return resp, nil
}

// PostJSON sends a POST request with a JSON-encoded body and decodes the
// response as JSON into out. If body is nil the request has no body. If out is
// nil the response body is not decoded. Content-Type is set to
// application/json automatically.
func (c *Client) PostJSON(rawURL string, body any, out any, opts ...RequestOption) (*Response, error) {
	return c.PostJSONCtx(context.Background(), rawURL, body, out, opts...)
}

// PostFormCtx sends a context-aware POST request with URL-encoded form data and
// decodes the response as JSON into out. Content-Type is set to
// application/x-www-form-urlencoded automatically.
func (c *Client) PostFormCtx(ctx context.Context, rawURL string, form map[string]string, out any, opts ...RequestOption) (*Response, error) {
	vals := url.Values{}
	for k, v := range form {
		vals.Set(k, v)
	}
	allOpts := append([]RequestOption{WithHeader("Content-Type", "application/x-www-form-urlencoded")}, opts...)
	resp, err := c.DoCtx(ctx, http.MethodPost, rawURL, strings.NewReader(vals.Encode()), allOpts...)
	if err != nil {
		return nil, err
	}
	if out != nil {
		resp.JSONErr = json.Unmarshal(resp.Body, out)
	}
	return resp, nil
}

// PostForm sends a POST request with URL-encoded form data and decodes the
// response as JSON into out. Content-Type is set to
// application/x-www-form-urlencoded automatically.
func (c *Client) PostForm(rawURL string, form map[string]string, out any, opts ...RequestOption) (*Response, error) {
	return c.PostFormCtx(context.Background(), rawURL, form, out, opts...)
}
