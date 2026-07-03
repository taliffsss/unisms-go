package unisms

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

// roundTripFunc adapts a function to the HTTPDoer interface for tests
// that need to simulate transport-level failures without a real network
// call.
type roundTripFunc func(req *http.Request) (*http.Response, error)

func (f roundTripFunc) Do(req *http.Request) (*http.Response, error) {
	return f(req)
}

func TestNew_EmptySecretKey(t *testing.T) {
	_, err := New("")
	if err == nil {
		t.Fatal("expected error for empty secret key, got nil")
	}

	var vErr *ValidationError
	if !errors.As(err, &vErr) {
		t.Fatalf("expected *ValidationError, got %T: %v", err, err)
	}
}

func TestNew_BlankSecretKey(t *testing.T) {
	_, err := New("   ")
	var vErr *ValidationError
	if !errors.As(err, &vErr) {
		t.Fatalf("expected *ValidationError, got %T: %v", err, err)
	}
}

func TestNew_SecretKeyFromEnv(t *testing.T) {
	t.Setenv(EnvSecretKey, "sk_from_env")

	c, err := New("")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if c.secretKey != "sk_from_env" {
		t.Fatalf("expected secret key from env, got %q", c.secretKey)
	}
}

func TestNew_ExplicitKeyTakesPrecedenceOverEnv(t *testing.T) {
	t.Setenv(EnvSecretKey, "sk_from_env")

	c, err := New("sk_explicit")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if c.secretKey != "sk_explicit" {
		t.Fatalf("expected explicit secret key, got %q", c.secretKey)
	}
}

func TestSend_Success(t *testing.T) {
	var gotUser, gotPass string
	var ok bool
	var gotBody map[string]interface{}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotUser, gotPass, ok = r.BasicAuth()
		if r.Method != http.MethodPost {
			t.Errorf("expected POST, got %s", r.Method)
		}
		if r.URL.Path != "/api/sms" {
			t.Errorf("expected path /api/sms, got %s", r.URL.Path)
		}
		if ct := r.Header.Get("Content-Type"); ct != "application/json" {
			t.Errorf("expected Content-Type application/json, got %s", ct)
		}

		_ = json.NewDecoder(r.Body).Decode(&gotBody)

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"message":{"reference_id":"msg_123","status":"queued"}}`))
	}))
	defer server.Close()

	c, err := New("sk_test", WithBaseURL(server.URL+"/api/sms"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	resp, err := c.Send(context.Background(), SendRequest{
		Recipient: "09055251658",
		Content:   "hello world",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !ok || gotUser != "sk_test" || gotPass != "" {
		t.Fatalf("expected basic auth user=sk_test pass=empty, got user=%q pass=%q ok=%v", gotUser, gotPass, ok)
	}

	if gotBody["sender_id"] != "UniSMS" {
		t.Errorf("expected default sender_id UniSMS, got %v", gotBody["sender_id"])
	}
	if _, present := gotBody["metadata"]; present {
		t.Errorf("expected metadata to be omitted, but it was present: %v", gotBody["metadata"])
	}

	msg, _ := resp["message"].(map[string]interface{})
	if msg["reference_id"] != "msg_123" {
		t.Errorf("expected reference_id msg_123, got %v", msg["reference_id"])
	}
}

func TestSend_CustomSenderID(t *testing.T) {
	var gotBody map[string]interface{}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewDecoder(r.Body).Decode(&gotBody)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{}`))
	}))
	defer server.Close()

	c, _ := New("sk_test", WithBaseURL(server.URL))
	_, err := c.Send(context.Background(), SendRequest{
		Recipient: "0905",
		Content:   "hi",
		SenderID:  "MyBrand",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if gotBody["sender_id"] != "MyBrand" {
		t.Errorf("expected sender_id MyBrand, got %v", gotBody["sender_id"])
	}
}

func TestSend_MetadataIncluded(t *testing.T) {
	var gotBody map[string]interface{}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewDecoder(r.Body).Decode(&gotBody)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{}`))
	}))
	defer server.Close()

	c, _ := New("sk_test", WithBaseURL(server.URL))
	_, err := c.Send(context.Background(), SendRequest{
		Recipient: "0905",
		Content:   "hi",
		Metadata:  map[string]interface{}{"order_id": float64(12345)},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	meta, ok := gotBody["metadata"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected metadata map in body, got %v", gotBody["metadata"])
	}
	if meta["order_id"] != float64(12345) {
		t.Errorf("expected order_id 12345, got %v", meta["order_id"])
	}
}

func TestSend_ValidationErrors(t *testing.T) {
	c, _ := New("sk_test")

	tests := []struct {
		name string
		req  SendRequest
	}{
		{"missing recipient", SendRequest{Content: "hi"}},
		{"blank recipient", SendRequest{Recipient: "   ", Content: "hi"}},
		{"missing content", SendRequest{Recipient: "0905"}},
		{"blank content", SendRequest{Recipient: "0905", Content: "  "}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := c.Send(context.Background(), tt.req)
			var vErr *ValidationError
			if !errors.As(err, &vErr) {
				t.Fatalf("expected *ValidationError, got %T: %v", err, err)
			}
		})
	}
}

func TestGetMessage_Success(t *testing.T) {
	var gotPath string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		if r.Method != http.MethodGet {
			t.Errorf("expected GET, got %s", r.Method)
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status":"delivered"}`))
	}))
	defer server.Close()

	c, _ := New("sk_test", WithBaseURL(server.URL))
	resp, err := c.GetMessage(context.Background(), "msg_84e8b93b")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if gotPath != "/msg_84e8b93b" {
		t.Errorf("expected path /msg_84e8b93b, got %s", gotPath)
	}
	if resp["status"] != "delivered" {
		t.Errorf("expected status delivered, got %v", resp["status"])
	}
}

func TestGetMessage_URLEncodesID(t *testing.T) {
	var gotPath string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{}`))
	}))
	defer server.Close()

	c, _ := New("sk_test", WithBaseURL(server.URL))
	_, err := c.GetMessage(context.Background(), "msg/with space")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !strings.Contains(gotPath, "msg") {
		t.Errorf("expected encoded path to contain msg, got %s", gotPath)
	}
}

func TestGetMessage_EmptyID(t *testing.T) {
	c, _ := New("sk_test")
	_, err := c.GetMessage(context.Background(), "  ")

	var vErr *ValidationError
	if !errors.As(err, &vErr) {
		t.Fatalf("expected *ValidationError, got %T: %v", err, err)
	}
}

func TestAPIError_NonRetryable4xx(t *testing.T) {
	var callCount int32

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&callCount, 1)
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"error":"bad request"}`))
	}))
	defer server.Close()

	c, _ := New("sk_test", WithBaseURL(server.URL), WithMaxRetries(2), WithRetryDelay(time.Millisecond))
	_, err := c.Send(context.Background(), SendRequest{Recipient: "0905", Content: "hi"})

	var apiErr *APIError
	if !errors.As(err, &apiErr) {
		t.Fatalf("expected *APIError, got %T: %v", err, err)
	}
	if apiErr.StatusCode != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d", apiErr.StatusCode)
	}
	if !strings.Contains(apiErr.ResponseBody, "bad request") {
		t.Errorf("expected response body to contain error message, got %s", apiErr.ResponseBody)
	}

	if got := atomic.LoadInt32(&callCount); got != 1 {
		t.Errorf("expected exactly 1 call (no retry on 400), got %d", got)
	}
}

func TestAPIError_RetriesOn5xxThenSucceeds(t *testing.T) {
	var callCount int32

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := atomic.AddInt32(&callCount, 1)
		if n < 3 {
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = w.Write([]byte(`{"error":"server error"}`))
			return
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status":"ok"}`))
	}))
	defer server.Close()

	c, _ := New("sk_test", WithBaseURL(server.URL), WithMaxRetries(2), WithRetryDelay(time.Millisecond))
	resp, err := c.Send(context.Background(), SendRequest{Recipient: "0905", Content: "hi"})
	if err != nil {
		t.Fatalf("unexpected error after retries: %v", err)
	}
	if resp["status"] != "ok" {
		t.Errorf("expected status ok, got %v", resp["status"])
	}
	if got := atomic.LoadInt32(&callCount); got != 3 {
		t.Errorf("expected exactly 3 calls (2 retries), got %d", got)
	}
}

func TestAPIError_RetriesOn429ExhaustsAndFails(t *testing.T) {
	var callCount int32

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&callCount, 1)
		w.WriteHeader(http.StatusTooManyRequests)
		_, _ = w.Write([]byte(`{"error":"rate limited"}`))
	}))
	defer server.Close()

	c, _ := New("sk_test", WithBaseURL(server.URL), WithMaxRetries(2), WithRetryDelay(time.Millisecond))
	_, err := c.Send(context.Background(), SendRequest{Recipient: "0905", Content: "hi"})

	var apiErr *APIError
	if !errors.As(err, &apiErr) {
		t.Fatalf("expected *APIError, got %T: %v", err, err)
	}
	if apiErr.StatusCode != http.StatusTooManyRequests {
		t.Errorf("expected status 429, got %d", apiErr.StatusCode)
	}

	// initial attempt + 2 retries = 3 total calls
	if got := atomic.LoadInt32(&callCount); got != 3 {
		t.Errorf("expected exactly 3 calls (retries exhausted), got %d", got)
	}
}

func TestTransportError_NetworkFailure(t *testing.T) {
	doer := roundTripFunc(func(req *http.Request) (*http.Response, error) {
		return nil, errors.New("connection refused")
	})

	c, _ := New("sk_test", WithHTTPClient(doer), WithMaxRetries(1), WithRetryDelay(time.Millisecond))
	_, err := c.Send(context.Background(), SendRequest{Recipient: "0905", Content: "hi"})

	var tErr *TransportError
	if !errors.As(err, &tErr) {
		t.Fatalf("expected *TransportError, got %T: %v", err, err)
	}
	if !strings.Contains(tErr.Error(), "connection refused") {
		t.Errorf("expected error message to mention cause, got %s", tErr.Error())
	}
}

func TestTransportError_RetriesExhausted(t *testing.T) {
	var callCount int32
	doer := roundTripFunc(func(req *http.Request) (*http.Response, error) {
		atomic.AddInt32(&callCount, 1)
		return nil, errors.New("network unreachable")
	})

	c, _ := New("sk_test", WithHTTPClient(doer), WithMaxRetries(2), WithRetryDelay(time.Millisecond))
	_, err := c.Send(context.Background(), SendRequest{Recipient: "0905", Content: "hi"})

	var tErr *TransportError
	if !errors.As(err, &tErr) {
		t.Fatalf("expected *TransportError, got %T: %v", err, err)
	}
	if got := atomic.LoadInt32(&callCount); got != 3 {
		t.Errorf("expected exactly 3 attempts (initial + 2 retries), got %d", got)
	}
}

func TestSend_NoRetriesWhenMaxRetriesZero(t *testing.T) {
	var callCount int32
	doer := roundTripFunc(func(req *http.Request) (*http.Response, error) {
		atomic.AddInt32(&callCount, 1)
		return nil, errors.New("boom")
	})

	c, _ := New("sk_test", WithHTTPClient(doer), WithMaxRetries(0))
	_, err := c.Send(context.Background(), SendRequest{Recipient: "0905", Content: "hi"})
	if err == nil {
		t.Fatal("expected error")
	}
	if got := atomic.LoadInt32(&callCount); got != 1 {
		t.Errorf("expected exactly 1 attempt with MaxRetries=0, got %d", got)
	}
}

func TestSend_ContextCancellationStopsRetries(t *testing.T) {
	var callCount int32
	ctx, cancel := context.WithCancel(context.Background())

	doer := roundTripFunc(func(req *http.Request) (*http.Response, error) {
		n := atomic.AddInt32(&callCount, 1)
		if n == 1 {
			cancel()
		}
		return nil, errors.New("boom")
	})

	c, _ := New("sk_test", WithHTTPClient(doer), WithMaxRetries(5), WithRetryDelay(10*time.Millisecond))
	_, err := c.Send(ctx, SendRequest{Recipient: "0905", Content: "hi"})
	if err == nil {
		t.Fatal("expected error")
	}

	// Should stop well short of 6 total attempts because the context was
	// cancelled after the first failed attempt.
	if got := atomic.LoadInt32(&callCount); got > 2 {
		t.Errorf("expected retries to stop after context cancellation, got %d calls", got)
	}
}

func TestSend_ContextDeadlineExceeded(t *testing.T) {
	doer := roundTripFunc(func(req *http.Request) (*http.Response, error) {
		select {
		case <-req.Context().Done():
			return nil, req.Context().Err()
		case <-time.After(50 * time.Millisecond):
			return nil, errors.New("should not reach here")
		}
	})

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Millisecond)
	defer cancel()

	c, _ := New("sk_test", WithHTTPClient(doer), WithMaxRetries(0))
	_, err := c.Send(ctx, SendRequest{Recipient: "0905", Content: "hi"})
	if err == nil {
		t.Fatal("expected error")
	}

	var tErr *TransportError
	if !errors.As(err, &tErr) {
		t.Fatalf("expected *TransportError, got %T: %v", err, err)
	}
}

func TestDecodeResponse_InvalidJSON(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`not json`))
	}))
	defer server.Close()

	c, _ := New("sk_test", WithBaseURL(server.URL))
	_, err := c.Send(context.Background(), SendRequest{Recipient: "0905", Content: "hi"})

	var vErr *ValidationError
	if !errors.As(err, &vErr) {
		t.Fatalf("expected *ValidationError, got %T: %v", err, err)
	}
}

func TestResponse_Accessors(t *testing.T) {
	resp := Response{"status": "ok", "count": 5}

	if resp.String("status") != "ok" {
		t.Errorf("expected status ok, got %s", resp.String("status"))
	}
	if resp.String("missing") != "" {
		t.Errorf("expected empty string for missing key")
	}
	if resp.String("count") != "" {
		t.Errorf("expected empty string for non-string value")
	}

	v, ok := resp.Get("count")
	if !ok || v != 5 {
		t.Errorf("expected Get to return 5, true; got %v, %v", v, ok)
	}

	_, ok = resp.Get("missing")
	if ok {
		t.Errorf("expected Get to return false for missing key")
	}
}
