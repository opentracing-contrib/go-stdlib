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
	req, ht := TraceRequest(tr, req)
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

	tests := []struct {
		url string
		num int
	}{
		{"/ok", 3},
		{"/redirect", 4},
		{"/fail", 3},
	}

	for _, tt := range tests {
		spans := makeRequest(t, srv.URL+tt.url)
		if got, want := len(spans), tt.num; got != want {
			t.Fatalf("got %d spans, expected %d", got, want)
		}
	}
}
