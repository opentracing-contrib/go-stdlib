package nethttp

import (
	"net/http"
	"net/http/httptest"
	"testing"

	opentracing "github.com/opentracing/opentracing-go"
	"github.com/opentracing/opentracing-go/mocktracer"
)

func makeRequest(t *testing.T, url string) []*mocktracer.MockSpan {
	tr := &mocktracer.MockTracer{}
	span := tr.StartSpan("toplevel")
	client := &http.Client{Transport: &Transport{}}
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		t.Fatal(err)
	}
	req = req.WithContext(opentracing.ContextWithSpan(req.Context(), span))
	req = TraceRequest(tr, req)
	resp, err := client.Do(req)
	if err == nil {
		_ = resp.Body.Close()
	}

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

	tests := []struct {
		url string
		num int
	}{
		{srv.URL + "/ok", 3},
		{srv.URL + "/redirect", 4},
		{srv.URL + "/fail", 3},
		{"http://localhost:0", 3},
	}

	for _, tt := range tests {
		spans := makeRequest(t, tt.url)
		if got, want := len(spans), tt.num; got != want {
			t.Fatalf("GET %s: got %d spans, expected %d", tt.url, got, want)
		}
	}
}
