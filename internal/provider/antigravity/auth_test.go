package antigravity

import (
	"context"
	"errors"
	"io"
	"net/http/httptest"
	"testing"
	"time"
)

func TestRunAuthFlow_ReturnsParentCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := RunAuthFlow(ctx, io.Discard, true)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("RunAuthFlow error = %v, want wrapped context.Canceled", err)
	}
	if err == context.Canceled {
		t.Fatal("RunAuthFlow should add authorization flow context to the cancellation error")
	}
}

func TestOAuthCallbackHandler_FirstResultWinsWithoutBlockingRepeats(t *testing.T) {
	resultCh := make(chan callbackResult, 1)
	handler := newOAuthCallbackHandler(resultCh)

	handler.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest("GET", "/?code=first", nil))

	secondDone := make(chan struct{})
	go func() {
		handler.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest("GET", "/?code=second", nil))
		close(secondDone)
	}()

	select {
	case <-secondDone:
	case <-time.After(time.Second):
		t.Fatal("repeated callback blocked while the first result was queued")
	}

	result := <-resultCh
	if result.err != nil {
		t.Fatalf("first callback returned error: %v", result.err)
	}
	if result.code != "first" {
		t.Errorf("callback code = %q, want first callback code", result.code)
	}

	handler.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest("GET", "/?code=third", nil))
	select {
	case extra := <-resultCh:
		t.Fatalf("later callback produced an extra result: %+v", extra)
	default:
	}
}
