package git

import (
	"context"
	"errors"
	"io"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"
)

// Native Git only talks to this private loopback endpoint. The fixed HTTPS
// upstream is fetched by Go, so the received-byte limit is enforced in-stream,
// rather than by periodically checking an already oversized pack on disk.
type transport struct {
	client    *http.Client
	upstream  string
	maximum   int64
	mu        sync.Mutex
	requestMu sync.Mutex
	received  int64
	attempts  int
	reason    string
}

func (t *transport) failed(code string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.reason == "" {
		t.reason = code
	}
}
func (t *transport) state() (int64, string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.received, t.reason
}

func (t *transport) serve(ctx context.Context) (string, func(), error) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return "", nil, err
	}
	server := &http.Server{Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.requestMu.Lock()
		defer t.requestMu.Unlock()
		suffix := strings.TrimPrefix(r.URL.Path, "/repo.git")
		if (r.Method != "GET" || suffix != "/info/refs" || r.URL.RawQuery != "service=git-upload-pack") &&
			(r.Method != "POST" || suffix != "/git-upload-pack" || r.URL.RawQuery != "") {
			http.Error(w, "unsupported Git operation", http.StatusBadRequest)
			return
		}
		t.mu.Lock()
		t.attempts++
		attemptLimit := t.attempts > 20
		exhausted := t.received >= t.maximum
		t.mu.Unlock()
		if attemptLimit {
			t.failed("GIT_HTTP_LIMIT")
			http.Error(w, "Git budget exceeded", 502)
			return
		}
		if exhausted {
			t.failed("GIT_SIZE_LIMIT")
			http.Error(w, "Git budget exceeded", 502)
			return
		}
		body := http.MaxBytesReader(w, r.Body, 8<<20)
		url := t.upstream + suffix
		if r.URL.RawQuery != "" {
			url += "?" + r.URL.RawQuery
		}
		attempt, cancel := context.WithTimeout(ctx, 15*time.Second)
		defer cancel()
		request, err := http.NewRequestWithContext(attempt, r.Method, url, body)
		if err != nil {
			t.failed("GIT_FETCH_FAILED")
			http.Error(w, "Git unavailable", 502)
			return
		}
		if r.ContentLength > 8<<20 {
			t.failed("GIT_REQUEST_LIMIT")
			http.Error(w, "Git budget exceeded", 502)
			return
		}
		request.ContentLength = r.ContentLength
		for _, name := range []string{"Content-Type", "Accept", "Git-Protocol"} {
			if value := r.Header.Get(name); value != "" {
				request.Header.Set(name, value)
			}
		}
		response, err := t.client.Do(request)
		if err != nil {
			if errors.Is(attempt.Err(), context.DeadlineExceeded) {
				t.failed("GIT_TIME_LIMIT")
			} else {
				t.failed("GIT_FETCH_FAILED")
			}
			http.Error(w, "Git unavailable", 502)
			return
		}
		defer response.Body.Close()
		if response.StatusCode != 200 {
			t.failed("GIT_FETCH_FAILED")
			http.Error(w, "Git unavailable", 502)
			return
		}
		t.mu.Lock()
		remaining := t.maximum - t.received
		t.mu.Unlock()
		if response.ContentLength > remaining {
			t.failed("GIT_SIZE_LIMIT")
			http.Error(w, "Git budget exceeded", 502)
			return
		}
		w.Header().Set("Content-Type", response.Header.Get("Content-Type"))
		responseRead := int64(0)
		buffer := make([]byte, 32<<10)
		for {
			t.mu.Lock()
			remaining = t.maximum - t.received
			t.mu.Unlock()
			if remaining <= 0 {
				t.failed("GIT_SIZE_LIMIT")
				panic(http.ErrAbortHandler)
			}
			chunk := buffer
			if int64(len(chunk)) > remaining {
				chunk = chunk[:int(remaining)]
			}
			n, readErr := response.Body.Read(chunk)
			if n > 0 {
				t.mu.Lock()
				t.received += int64(n)
				responseRead += int64(n)
				t.mu.Unlock()
				if _, err := w.Write(chunk[:n]); err != nil {
					return
				}
				if flush, ok := w.(http.Flusher); ok {
					flush.Flush()
				}
			}
			if response.ContentLength >= 0 && responseRead >= response.ContentLength {
				return
			}
			if errors.Is(readErr, io.EOF) {
				return
			}
			if readErr != nil {
				if errors.Is(attempt.Err(), context.DeadlineExceeded) {
					t.failed("GIT_TIME_LIMIT")
				} else {
					t.failed("GIT_FETCH_INTERRUPTED")
				}
				panic(http.ErrAbortHandler)
			}
		}
	})}
	go func() { _ = server.Serve(listener) }()
	return "http://" + listener.Addr().String() + "/repo.git", func() { _ = server.Close() }, nil
}
