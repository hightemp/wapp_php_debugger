package source_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/hightemp/wapp_php_debugger/server/internal/libclient/libtest"
	"github.com/hightemp/wapp_php_debugger/server/internal/source"
	"github.com/hightemp/wapp_php_debugger/server/internal/source/sourcetest"
)

const libToken = "0123456789abcdef0123456789abcdef"

func newHTTP(t *testing.T, files map[string][]byte) (*source.HTTP, *libtest.Server) {
	t.Helper()
	server := libtest.Start(t, libToken, files)
	src, err := source.NewHTTP(source.HTTPOptions{BaseURL: server.URL, Token: libToken, Client: server.Client()})
	if err != nil {
		t.Fatal(err)
	}
	return src, server
}

func TestHTTPContract(t *testing.T) {
	sourcetest.Run(t, func(t *testing.T, files map[string][]byte) source.Source {
		src, _ := newHTTP(t, files)
		return src
	})
}

func TestHTTPWithoutTheLibraryAnswerIsNotAMissingFile(t *testing.T) {
	server := libtest.Start(t, libToken, map[string][]byte{"1792400000000": []byte("x")})
	src, err := source.NewHTTP(source.HTTPOptions{BaseURL: server.URL, Token: "wrong-token-wrong-token-wrong-tok", Client: server.Client()})
	if err != nil {
		t.Fatal(err)
	}
	_, err = src.List(context.Background())
	if !errors.Is(err, source.ErrNoLibrary) {
		t.Fatalf("err = %v, want ErrNoLibrary", err)
	}
	if _, err := src.ReadAt(context.Background(), "1792400000000", 0, 1); errors.Is(err, source.ErrNotFound) || !errors.Is(err, source.ErrNoLibrary) {
		t.Fatalf("ReadAt err = %v, want ErrNoLibrary", err)
	}
}

func TestHTTPWaitsOutTheRateLimit(t *testing.T) {
	src, server := newHTTP(t, nil)
	server.Limited.Store(1)
	ctx := context.Background()
	if _, err := src.List(ctx); !errors.Is(err, source.ErrRateLimited) {
		t.Fatalf("err = %v, want ErrRateLimited", err)
	}
	before := server.Requests()
	if _, err := src.List(ctx); !errors.Is(err, source.ErrRateLimited) {
		t.Fatalf("within Retry-After: err = %v", err)
	}
	if server.Requests() != before {
		t.Fatal("a request went out within Retry-After")
	}
	time.Sleep(1100 * time.Millisecond)
	if _, err := src.List(ctx); err != nil {
		t.Fatalf("after Retry-After: %v", err)
	}
}

func TestHTTPStatusOfTheLibrary(t *testing.T) {
	src, _ := newHTTP(t, nil)
	status, err := src.Status(context.Background())
	if err != nil || status.LibraryVersion != "2.0.0" || status.PhpVersion == nil || *status.PhpVersion != "7.4.33" || !status.LogsDirWritable {
		t.Fatalf("status = %+v, %v", status, err)
	}
}

func TestHTTPNeedsAnAbsoluteURL(t *testing.T) {
	for _, base := range []string{"", "shop.example.test", "ftp://shop.example.test", "https://"} {
		if _, err := source.NewHTTP(source.HTTPOptions{BaseURL: base, Token: libToken}); err == nil {
			t.Errorf("NewHTTP(%q) accepted", base)
		}
	}
}

func TestHTTPReportsTheClockOfTheServer(t *testing.T) {
	server := libtest.Start(t, libToken, nil)
	server.ClockAhead.Store(int64(40 * time.Second))
	var offsets []time.Duration
	src, err := source.NewHTTP(source.HTTPOptions{BaseURL: server.URL, Token: libToken, Client: server.Client(),
		OnClock: func(offset time.Duration) { offsets = append(offsets, offset) }})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := src.List(context.Background()); err != nil {
		t.Fatal(err)
	}
	if len(offsets) != 1 || offsets[0] < 39*time.Second || offsets[0] > 41*time.Second {
		t.Fatalf("offsets = %v; want about 40s", offsets)
	}
}
