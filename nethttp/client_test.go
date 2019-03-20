package nethttp

import (
	"net/http"
	"net/http/httptest"
	"testing"

	opentracing "github.com/opentracing/opentracing-go"
	"github.com/opentracing/opentracing-go/ext"
	"github.com/opentracing/opentracing-go/mocktracer"
)

func makeRequest(t *testing.T, url string, options ...ClientOption) []*mocktracer.MockSpan {
	tr := &mocktracer.MockTracer{}
	span := tr.StartSpan("toplevel")
	client := &http.Client{Transport: &Transport{}}
	req, err := http.NewRequest("GET", url, nil)
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
	mux := http.NewServeMux()
	mux.HandleFunc("/ok", func(w http.ResponseWriter, r *http.Request) {})
	mux.HandleFunc("/redirect", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/ok", http.StatusTemporaryRedirect)
	})
	mux.HandleFunc("/fail", func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "failure", http.StatusInternalServerError)
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	helloWorldObserver := func(s opentracing.Span, r *http.Request) {
		s.SetTag("hello", "world")
	}

	tests := []struct {
		url          string
		num          int
		opts         []ClientOption
		opName       string
		expectedTags map[string]interface{}
	}{
		{url: "/ok", num: 3, opts: nil, opName: "HTTP Client"},
		{url: "/redirect", num: 4, opts: []ClientOption{OperationName("client-span")}, opName: "client-span"},
		{url: "/fail", num: 3, opts: nil, opName: "HTTP Client", expectedTags: makeTags(string(ext.Error), true)},
		{url: "/ok", num: 3, opts: []ClientOption{ClientSpanObserver(helloWorldObserver)}, opName: "HTTP Client", expectedTags: makeTags("hello", "world")},
	}

	for _, tt := range tests {
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
	}
}

func TestTracerFromRequest(t *testing.T) {
	req, err := http.NewRequest("GET", "foobar", nil)
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

func makeTags(keyVals ...interface{}) map[string]interface{} {
	result := make(map[string]interface{}, len(keyVals)/2)
	for i := 0; i < len(keyVals)-1; i += 2 {
		key := keyVals[i].(string)
		result[key] = keyVals[i+1]
	}
	return result
}
