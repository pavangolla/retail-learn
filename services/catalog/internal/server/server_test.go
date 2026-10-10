package server

import (
	"context"
	"encoding/json"
	"errors"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

type fakePinger struct{ err error }

func (f fakePinger) Ping(context.Context) error { return f.err }

func TestHealth(t *testing.T) {
	tests := []struct {
		name       string
		pingErr    error
		wantCode   int
		wantStatus string
		wantMongo  string
	}{
		{"mongo up", nil, http.StatusOK, "ok", "up"},
		{"mongo down", errors.New("connection refused"), http.StatusServiceUnavailable, "degraded", "down"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			h := New(fakePinger{err: tc.pingErr})
			rec := httptest.NewRecorder()
			h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/health", nil))

			if rec.Code != tc.wantCode {
				t.Fatalf("status code = %d, want %d", rec.Code, tc.wantCode)
			}
			var got healthResponse
			if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
				t.Fatalf("bad JSON: %v", err)
			}
			if got.Status != tc.wantStatus || got.Mongo != tc.wantMongo || got.Service != "catalog" {
				t.Fatalf("body = %+v", got)
			}
		})
	}
}

func TestHealthRejectsOtherMethods(t *testing.T) {
	h := New(fakePinger{})
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/health", nil))
	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("POST /health = %d, want 405", rec.Code)
	}
}

func TestTCPPinger(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	addr := ln.Addr().String()

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	if err := (TCPPinger{Addr: addr}).Ping(ctx); err != nil {
		t.Fatalf("listening port should be reachable: %v", err)
	}

	ln.Close()
	if err := (TCPPinger{Addr: addr}).Ping(ctx); err == nil {
		t.Fatal("closed port should fail")
	}
}
