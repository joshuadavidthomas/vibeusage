package httpclient

import "net/http"

// WithHeader sets a request header.
func WithHeader(key, value string) RequestOption {
	return func(r *http.Request) {
		r.Header.Set(key, value)
	}
}

// WithBearer sets the Authorization header to "Bearer <token>".
func WithBearer(token string) RequestOption {
	return func(r *http.Request) {
		r.Header.Set("Authorization", "Bearer "+token)
	}
}

// WithCookie adds a cookie to the request.
func WithCookie(name, value string) RequestOption {
	return func(r *http.Request) {
		r.AddCookie(&http.Cookie{Name: name, Value: value})
	}
}
