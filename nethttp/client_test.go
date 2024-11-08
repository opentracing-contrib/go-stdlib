package nethttp

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	opentracing "github.com/opentracing/opentracing-go"
	"github.com/opentracing/opentracing-go/ext"
	"github.com/opentracing/opentracing-go/mocktracer"
)

func makeRequest(t *testing.T, url string, options ...ClientOption) []*mocktracer.MockSpan {
	t.Helper()
	tr := &mocktracer.MockTracer{}
	span := tr.StartSpan("toplevel")
	client := &http.Client{Transport: &Transport{}}
	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, url, nil)
	if err != nil {
		t.Fatal(err)
	}
	req = req.WithContext(opentracing.ContextWithSpan(req.Context(), span))
	req, ht := TraceRequest(tr, req, options...)
	resp, err := client.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	_ = resp.Body.Close()
	ht.Finish()
	span.Finish()

	return tr.FinishedSpans()
}

func TestClientTrace(t *testing.T) {
	t.Parallel()
	mux := http.NewServeMux()
	mux.HandleFunc("/ok", func(w http.ResponseWriter, r *http.Request) {})
	mux.HandleFunc("/redirect", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/ok", http.StatusTemporaryRedirect)
	})
	mux.HandleFunc("/fail", func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "failure", http.StatusInternalServerError)
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	helloWorldObserver := func(s opentracing.Span, r *http.Request) {
		s.SetTag("hello", "world")
	}

	tests := []struct {
		expectedTags map[string]interface{}
		url          string
		opName       string
		opts         []ClientOption
		num          int
	}{
		{url: "/ok", num: 3, opts: nil, opName: "HTTP Client"},
		{url: "/redirect", num: 4, opts: []ClientOption{OperationName("client-span")}, opName: "client-span"},
		{url: "/fail", num: 3, opts: nil, opName: "HTTP Client", expectedTags: makeTags(t, string(ext.Error), true)},
		{url: "/ok", num: 3, opts: []ClientOption{ClientSpanObserver(helloWorldObserver)}, opName: "HTTP Client", expectedTags: makeTags(t, "hello", "world")},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.url, func(t *testing.T) {
			t.Parallel()
			t.Log(tt.opName)
			spans := makeRequest(t, srv.URL+tt.url, tt.opts...)
			if got, want := len(spans), tt.num; got != want {
				t.Fatalf("got %d spans, expected %d", got, want)
			}
			var rootSpan *mocktracer.MockSpan
			for _, span := range spans {
				if span.ParentID == 0 {
					rootSpan = span
					break
				}
			}
			if rootSpan == nil {
				t.Fatal("cannot find root span with ParentID==0")
			}

			foundClientSpan := false
			for _, span := range spans {
				if span.ParentID == rootSpan.SpanContext.SpanID {
					foundClientSpan = true
					if got, want := span.OperationName, tt.opName; got != want {
						t.Fatalf("got %s operation name, expected %s", got, want)
					}
				}
				if span.OperationName == "HTTP GET" {
					logs := span.Logs()
					if len(logs) < 6 {
						t.Fatalf("got %d, expected at least %d log events", len(logs), 6)
					}

					key := logs[0].Fields[0].Key
					if key != "event" {
						t.Fatalf("got %s, expected %s", key, "event")
					}
					v := logs[0].Fields[0].ValueString
					if v != "GetConn" {
						t.Fatalf("got %s, expected %s", v, "GetConn")
					}

					for k, expected := range tt.expectedTags {
						result := span.Tag(k)
						if expected != result {
							t.Fatalf("got %v, expected %v, for key %s", result, expected, k)
						}
					}
				}
			}
			if !foundClientSpan {
				t.Fatal("cannot find client span")
			}
		})
	}
}

func TestTracerFromRequest(t *testing.T) {
	t.Parallel()
	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, "foobar", nil)
	if err != nil {
		t.Fatal(err)
	}

	ht := TracerFromRequest(req)
	if ht != nil {
		t.Fatal("request should not have a tracer yet")
	}

	tr := &mocktracer.MockTracer{}
	req, expected := TraceRequest(tr, req)

	ht = TracerFromRequest(req)
	if ht != expected {
		t.Fatalf("got %v, expected %v", ht, expected)
	}
}

func TestWriteCloserFromRequest(t *testing.T) {
	t.Parallel()
	wait := make(chan bool)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			wait <- true
		}()

		w.Header().Set("Upgrade", "websocket")
		w.Header().Set("Connection", "Upgrade")
		w.WriteHeader(http.StatusSwitchingProtocols)

		hijacker, ok := w.(http.Hijacker)
		if !ok {
			t.Fatal("ResponseWriter does not implement http.Hijacker")
		}

		_, rw, err := hijacker.Hijack()
		if err != nil {
			t.Fatal("Failed to hijack connection")
		}

		line, _, err := rw.ReadLine()
		if string(line) != "ping" {
			t.Fatalf("Expected 'ping' received %q", string(line))
		}

		if err != nil {
			t.Fatal(err)
		}
	}))

	var buf bytes.Buffer
	req, err := http.NewRequestWithContext(context.Background(), http.MethodPost, srv.URL, &buf)
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Connection", "upgrade")
	req.Header.Set("Upgrade", "websocket")
	req.Proto = "HTTP/1.1"
	req.ProtoMajor = 1
	req.ProtoMinor = 1

	tr := &mocktracer.MockTracer{}
	req, _ = TraceRequest(tr, req)

	client := &http.Client{Transport: &Transport{}}
	resp, err := client.Do(req)
	if err != nil {
		t.Fatal(err)
	}

	rw, ok := resp.Body.(io.ReadWriteCloser)
	if !ok {
		t.Fatal("resp.Body is not a io.ReadWriteCloser")
	}

	fmt.Fprint(rw, "ping\n")
	<-wait
	rw.Close()
}

func TestInjectSpanContext(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name                     string
		opts                     []ClientOption
		expectContextPropagation bool
	}{
		{name: "Default", expectContextPropagation: true, opts: nil},
		{name: "True", expectContextPropagation: true, opts: []ClientOption{InjectSpanContext(true)}},
		{name: "False", expectContextPropagation: false, opts: []ClientOption{InjectSpanContext(false)}},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			var handlerCalled bool
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				handlerCalled = true
				srvTr := mocktracer.New()
				ctx, err := srvTr.Extract(opentracing.HTTPHeaders, opentracing.HTTPHeadersCarrier(r.Header))

				if err != nil && tt.expectContextPropagation {
					t.Fatal(err)
				}

				if tt.expectContextPropagation {
					if err != nil || ctx == nil {
						t.Fatal("expected propagation but unable to extract")
					}
				} else {
					// Expect "opentracing: SpanContext not found in Extract carrier" when not injected
					// Can't check ctx directly, because it gets set to emptyContext
					if err == nil {
						t.Fatal("unexpected propagation")
					}
				}
			}))

			tr := mocktracer.New()
			span := tr.StartSpan("root")

			req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, srv.URL, nil)
			if err != nil {
				t.Fatal(err)
			}
			req = req.WithContext(opentracing.ContextWithSpan(req.Context(), span))

			req, ht := TraceRequest(tr, req, tt.opts...)

			client := &http.Client{Transport: &Transport{}}
			resp, err := client.Do(req)
			if err != nil {
				t.Fatal(err)
			}
			_ = resp.Body.Close()

			ht.Finish()
			span.Finish()

			srv.Close()

			if !handlerCalled {
				t.Fatal("server handler never called")
			}
		})
	}
}

func makeTags(t *testing.T, keyVals ...interface{}) map[string]interface{} {
	t.Helper()
	result := make(map[string]interface{}, len(keyVals)/2)
	for i := 0; i < len(keyVals)-1; i += 2 {
		key, ok := keyVals[i].(string)
		if !ok {
			t.Fatalf("expected string key, got %T", keyVals[i])
		}
		result[key] = keyVals[i+1]
	}
	return result
}

func TestClientCustomURL(t *testing.T) {
	t.Parallel()
	mux := http.NewServeMux()
	mux.HandleFunc("/ok", func(w http.ResponseWriter, r *http.Request) {})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	fn := func(u *url.URL) string {
		// Simulate redacting token
		return srv.URL + u.Path + "?token=*"
	}

	tests := []struct {
		url  string
		tag  string
		opts []ClientOption
	}{
		// These first cases fail early
		{url: "/ok?token=a", tag: srv.URL + "/ok?token=a", opts: []ClientOption{}},
		{url: "/ok?token=c", tag: srv.URL + "/ok?token=*", opts: []ClientOption{URLTagFunc(fn)}},
		// Disable ClientTrace to fire RoundTrip
		{url: "/ok?token=b", tag: srv.URL + "/ok?token=b", opts: []ClientOption{ClientTrace(false)}},
		{url: "/ok?token=c", tag: srv.URL + "/ok?token=*", opts: []ClientOption{ClientTrace(false), URLTagFunc(fn)}},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.url, func(t *testing.T) {
			t.Parallel()
			var clientSpan *mocktracer.MockSpan

			spans := makeRequest(t, srv.URL+tt.url, tt.opts...)
			for _, span := range spans {
				if span.OperationName == "HTTP GET" {
					clientSpan = span
					break
				}
			}
			if clientSpan == nil {
				t.Fatal("cannot find client span")
			}
			tag := clientSpan.Tags()["http.url"]
			if got, want := tag, tt.tag; got != want {
				t.Fatalf("got %s tag name, expected %s", got, want)
			}
			peerAddress, ok := clientSpan.Tags()["peer.address"]
			if !ok {
				t.Fatal("cannot find peer.address tag")
			}
			if peerAddress != srv.Listener.Addr().String() {
				t.Fatalf("got %s want %s in peer.address tag", peerAddress, srv.Listener.Addr().String())
			}
		})
	}
}
