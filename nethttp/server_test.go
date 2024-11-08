package nethttp

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"reflect"
	"strings"
	"testing"

	"github.com/opentracing/opentracing-go"
	"github.com/opentracing/opentracing-go/ext"
	"github.com/opentracing/opentracing-go/mocktracer"
)

func TestOperationNameOption(t *testing.T) {
	t.Parallel()
	mux := http.NewServeMux()
	mux.HandleFunc("/root", func(w http.ResponseWriter, r *http.Request) {})

	fn := func(r *http.Request) string {
		return "HTTP " + r.Method + ": /root"
	}

	tests := []struct {
		opName  string
		options []MWOption
	}{
		{"HTTP GET", nil},
		{"HTTP GET: /root", []MWOption{OperationNameFunc(fn)}},
	}

	for _, tt := range tests {
		testCase := tt
		t.Run(testCase.opName, func(t *testing.T) {
			t.Parallel()
			tr := &mocktracer.MockTracer{}
			mw := Middleware(tr, mux, testCase.options...)
			srv := httptest.NewServer(mw)
			defer srv.Close()

			_, err := http.Get(srv.URL)
			if err != nil {
				t.Fatalf("server returned error: %v", err)
			}

			spans := tr.FinishedSpans()
			if got, want := len(spans), 1; got != want {
				t.Fatalf("got %d spans, expected %d", got, want)
			}

			if got, want := spans[0].OperationName, testCase.opName; got != want {
				t.Fatalf("got %s operation name, expected %s", got, want)
			}
		})
	}
}

func TestSpanObserverOption(t *testing.T) {
	t.Parallel()
	mux := http.NewServeMux()
	mux.HandleFunc("/root", func(w http.ResponseWriter, r *http.Request) {})

	opNamefn := func(r *http.Request) string {
		return "HTTP " + r.Method + ": /root"
	}
	spanObserverfn := func(sp opentracing.Span, r *http.Request) {
		sp.SetTag("http.uri", r.URL.EscapedPath())
	}
	wantTags := map[string]interface{}{"http.uri": "/"}

	tests := []struct {
		Tags    map[string]interface{}
		opName  string
		options []MWOption
	}{
		{nil, "HTTP GET", nil},
		{nil, "HTTP GET: /root", []MWOption{OperationNameFunc(opNamefn)}},
		{wantTags, "HTTP GET", []MWOption{MWSpanObserver(spanObserverfn)}},
		{wantTags, "HTTP GET: /root", []MWOption{OperationNameFunc(opNamefn), MWSpanObserver(spanObserverfn)}},
	}

	for _, tt := range tests {
		testCase := tt
		t.Run(testCase.opName, func(t *testing.T) {
			t.Parallel()
			tr := &mocktracer.MockTracer{}
			mw := Middleware(tr, mux, testCase.options...)
			srv := httptest.NewServer(mw)
			defer srv.Close()

			_, err := http.Get(srv.URL)
			if err != nil {
				t.Fatalf("server returned error: %v", err)
			}

			spans := tr.FinishedSpans()
			if got, want := len(spans), 1; got != want {
				t.Fatalf("got %d spans, expected %d", got, want)
			}

			if got, want := spans[0].OperationName, testCase.opName; got != want {
				t.Fatalf("got %s operation name, expected %s", got, want)
			}

			defaultLength := 6
			if len(spans[0].Tags()) != len(testCase.Tags)+defaultLength {
				t.Fatalf("got tag length %d, expected %d", len(spans[0].Tags()), len(testCase.Tags))
			}
			for k, v := range testCase.Tags {
				if tag, ok := spans[0].Tag(k).(string); !ok || v != tag {
					t.Fatalf("got %v tag, expected %v", tag, v)
				}
			}
		})
	}
}

//nolint:paralleltest,tparallel
func TestSpanFilterOption(t *testing.T) {
	t.Parallel()
	mux := http.NewServeMux()
	mux.HandleFunc("/root", func(w http.ResponseWriter, r *http.Request) {})

	spanFilterfn := func(r *http.Request) bool {
		return !strings.HasPrefix(r.Header.Get("User-Agent"), "kube-probe")
	}
	noAgentReq, _ := http.NewRequest(http.MethodGet, "/root", nil)
	noAgentReq.Header.Del("User-Agent")
	probeReq1, _ := http.NewRequest(http.MethodGet, "/root", nil)
	probeReq1.Header.Add("User-Agent", "kube-probe/1.12")
	probeReq2, _ := http.NewRequest(http.MethodGet, "/root", nil)
	probeReq2.Header.Add("User-Agent", "kube-probe/9.99")
	postmanReq, _ := http.NewRequest(http.MethodGet, "/root", nil)
	postmanReq.Header.Add("User-Agent", "PostmanRuntime/7.3.0")
	tests := []struct {
		request            *http.Request
		opName             string
		options            []MWOption
		ExpectToCreateSpan bool
	}{
		{noAgentReq, "No filter", nil, true},
		{noAgentReq, "No filter", []MWOption{MWSpanFilter(spanFilterfn)}, true},
		{probeReq1, "User-Agent: kube-probe/1.12", []MWOption{MWSpanFilter(spanFilterfn)}, false},
		{probeReq2, "User-Agent: kube-probe/9.99", []MWOption{MWSpanFilter(spanFilterfn)}, false},
		{postmanReq, "User-Agent: PostmanRuntime/7.3.0", []MWOption{MWSpanFilter(spanFilterfn)}, true},
	}

	for _, tt := range tests {
		testCase := tt
		t.Run(testCase.opName, func(t *testing.T) {
			tr := &mocktracer.MockTracer{}
			mw := Middleware(tr, mux, testCase.options...)
			srv := httptest.NewServer(mw)
			defer srv.Close()

			client := &http.Client{}
			testCase.request.URL, _ = url.Parse(srv.URL)
			_, err := client.Do(testCase.request)
			if err != nil {
				t.Fatalf("server returned error: %v", err)
			}

			spans := tr.FinishedSpans()
			if spanCreated := len(spans) == 1; spanCreated != testCase.ExpectToCreateSpan {
				t.Fatalf("spanCreated %t, ExpectToCreateSpan %t", spanCreated, testCase.ExpectToCreateSpan)
			}
		})
	}
}

func TestURLTagOption(t *testing.T) {
	t.Parallel()
	mux := http.NewServeMux()
	mux.HandleFunc("/root", func(w http.ResponseWriter, r *http.Request) {})

	fn := func(u *url.URL) string {
		// Log path only (no query parameters etc)
		return u.Path
	}

	tests := []struct {
		url     string
		tag     string
		options []MWOption
	}{
		{"/root?token=123", "/root?token=123", []MWOption{}},
		{"/root?token=123", "/root", []MWOption{MWURLTagFunc(fn)}},
	}

	for _, tt := range tests {
		testCase := tt
		t.Run(testCase.tag, func(t *testing.T) {
			t.Parallel()
			tr := &mocktracer.MockTracer{}
			mw := Middleware(tr, mux, testCase.options...)
			srv := httptest.NewServer(mw)
			defer srv.Close()

			_, err := http.Get(srv.URL + testCase.url)
			if err != nil {
				t.Fatalf("server returned error: %v", err)
			}

			spans := tr.FinishedSpans()
			if got, want := len(spans), 1; got != want {
				t.Fatalf("got %d spans, expected %d", got, want)
			}

			tag := spans[0].Tags()["http.url"]
			if got, want := tag, testCase.tag; got != want {
				t.Fatalf("got %s tag name, expected %s", got, want)
			}
		})
	}
}

func TestSpanErrorAndStatusCode(t *testing.T) {
	t.Parallel()
	mux := http.NewServeMux()
	mux.HandleFunc("/header-and-body", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		if _, err := w.Write([]byte("OK")); err != nil {
			t.Fatalf("failed to write response body: %v", err)
		}
	})
	mux.HandleFunc("/body-only", func(w http.ResponseWriter, r *http.Request) {
		if _, err := w.Write([]byte("OK")); err != nil {
			t.Fatalf("failed to write response body: %v", err)
		}
	})
	mux.HandleFunc("/header-only", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	mux.HandleFunc("/empty", func(w http.ResponseWriter, r *http.Request) {
		// no status header
	})
	mux.HandleFunc("/error", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	})

	expStatusOK := map[string]interface{}{"http.status_code": uint16(200)}

	tests := []struct {
		tags map[string]interface{}
		url  string
	}{
		{url: "/header-and-body", tags: expStatusOK},
		{url: "/body-only", tags: expStatusOK},
		{url: "/header-only", tags: expStatusOK},
		{url: "/empty", tags: expStatusOK},
		{url: "/error", tags: map[string]interface{}{"http.status_code": uint16(500), string(ext.Error): true}},
	}

	for _, tt := range tests {
		testCase := tt
		t.Run(testCase.url, func(t *testing.T) {
			t.Parallel()
			tr := &mocktracer.MockTracer{}
			mw := Middleware(tr, mux)
			srv := httptest.NewServer(mw)
			defer srv.Close()

			req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, srv.URL+testCase.url, nil)
			if err != nil {
				t.Fatalf("failed to create request: %v", err)
			}
			client := &http.Client{}
			resp, err := client.Do(req)
			if err != nil {
				t.Fatalf("server returned error: %v", err)
			}
			defer resp.Body.Close()

			spans := tr.FinishedSpans()
			if got, want := len(spans), 1; got != want {
				t.Fatalf("got %d spans, expected %d", got, want)
			}

			for k, v := range testCase.tags {
				if tag := spans[0].Tag(k); !reflect.DeepEqual(tag, v) {
					t.Fatalf("tag %s: got %v, expected %v", k, tag, v)
				}
			}
		})
	}
}

func TestSpanResponseSize(t *testing.T) {
	t.Parallel()
	mux := http.NewServeMux()
	mux.HandleFunc("/with-body", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		if _, err := w.Write([]byte("12345")); err != nil {
			t.Fatalf("failed to write response body: %v", err)
		}
	})
	mux.HandleFunc("/no-body", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	expBodySize := map[string]interface{}{"http.response_size": 5}

	tests := []struct {
		tags map[string]interface{}
		url  string
	}{
		{url: "/with-body", tags: expBodySize},
		{url: "/no-body", tags: map[string]interface{}{}},
	}

	for _, tt := range tests {
		testCase := tt
		t.Run(testCase.url, func(t *testing.T) {
			t.Parallel()
			tr := &mocktracer.MockTracer{}
			mw := Middleware(tr, mux)
			srv := httptest.NewServer(mw)
			defer srv.Close()

			req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, srv.URL+testCase.url, nil)
			if err != nil {
				t.Fatalf("failed to create request: %v", err)
			}
			client := &http.Client{}
			resp, err := client.Do(req)
			if err != nil {
				t.Fatalf("server returned error: %v", err)
			}
			defer resp.Body.Close()

			spans := tr.FinishedSpans()
			if got, want := len(spans), 1; got != want {
				t.Fatalf("got %d spans, expected %d", got, want)
			}

			for k, v := range testCase.tags {
				if tag := spans[0].Tag(k); !reflect.DeepEqual(tag, v) {
					t.Fatalf("tag %s: got %v, expected %v", k, tag, v)
				}
			}
		})
	}
}

func BenchmarkStatusCodeTrackingOverhead(b *testing.B) {
	mux := http.NewServeMux()
	mux.HandleFunc("/root", func(w http.ResponseWriter, r *http.Request) {})
	tr := &mocktracer.MockTracer{}
	mw := Middleware(tr, mux)
	srv := httptest.NewServer(mw)
	defer srv.Close()

	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			resp, err := http.Get(srv.URL)
			if err != nil {
				b.Fatalf("server returned error: %v", err)
			}
			err = resp.Body.Close()
			if err != nil {
				b.Fatalf("failed to close response: %v", err)
			}
		}
	})
}

func BenchmarkResponseSizeTrackingOverhead(b *testing.B) {
	mux := http.NewServeMux()
	mux.HandleFunc("/root", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		if _, err := w.Write([]byte("12345")); err != nil {
			b.Fatalf("failed to write response body: %v", err)
		}
	})
	tr := &mocktracer.MockTracer{}
	mw := Middleware(tr, mux)
	srv := httptest.NewServer(mw)
	defer srv.Close()

	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			resp, err := http.Get(srv.URL)
			if err != nil {
				b.Fatalf("server returned error: %v", err)
			}
			err = resp.Body.Close()
			if err != nil {
				b.Fatalf("failed to close response: %v", err)
			}
		}
	})
}

func TestMiddlewareHandlerPanic(t *testing.T) {
	t.Parallel()
	tests := []struct {
		handler func(w http.ResponseWriter, r *http.Request)
		name    string
		status  uint16
		isError bool
	}{
		{
			name: "OK",
			handler: func(w http.ResponseWriter, r *http.Request) {
				if _, err := w.Write([]byte("OK")); err != nil {
					t.Fatalf("failed to write response body: %v", err)
				}
			},
			status:  http.StatusOK,
			isError: false,
		},
		{
			name: "Panic",
			handler: func(w http.ResponseWriter, r *http.Request) {
				panic("panic test")
			},
			status:  0,
			isError: true,
		},
		{
			name: "InternalServerError",
			handler: func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusInternalServerError)
				if _, err := w.Write([]byte("InternalServerError")); err != nil {
					t.Fatalf("failed to write response body: %v", err)
				}
			},
			status:  http.StatusInternalServerError,
			isError: true,
		},
	}

	for _, tc := range tests {
		testCase := tc
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()
			mux := http.NewServeMux()
			mux.HandleFunc("/root", testCase.handler)
			tr := &mocktracer.MockTracer{}
			srv := httptest.NewServer(MiddlewareFunc(tr, mux.ServeHTTP))
			defer srv.Close()

			_, err := http.Get(srv.URL + "/root")
			if err != nil {
				t.Logf("server returned error: %v", err)
			}

			spans := tr.FinishedSpans()
			if got, want := len(spans), 1; got != want {
				t.Fatalf("got %d spans, expected %d", got, want)
			}
			actualStatus := spans[0].Tag(string(ext.HTTPStatusCode))
			if testCase.status > 0 && !reflect.DeepEqual(testCase.status, actualStatus) {
				t.Fatalf("got status code %v, expected %d", actualStatus, testCase.status)
			}
			actualErr, ok := spans[0].Tag(string(ext.Error)).(bool)
			if !ok {
				actualErr = false
			}
			if testCase.isError != actualErr {
				t.Fatalf("got span error %v, expected %v", actualErr, testCase.isError)
			}
		})
	}
}
