// +build go1.7

package nethttp

import (
	"context"
	"io"
	"net/http"
	"net/http/httptrace"

	"github.com/opentracing/opentracing-go"
	"github.com/opentracing/opentracing-go/ext"
)

type contextKey int

const (
	keyTracer contextKey = iota
)

// Transport wraps a RoundTripper. If a request is being traced with
// Tracer, Transport will inject the current span into the headers,
// set HTTP related tags on the span as well as finish the span after
// the response body is closed.
type Transport struct {
	http.RoundTripper
}

// TraceRequest adds a ClientTracer to req, tracing the request and
// all requests caused due to redirects. When tracing requests this
// way you must also use Transport.
//
// Example:
//
// 	http.DefaultClient.Transport = &nethttp.Transport{http.DefaultTransport}
// 	req, err := http.NewRequest("GET", "http://google.com", nil)
// 	if err != nil {
// 		log.Fatal(err)
// 	}
// 	req, ht := nethttp.TraceRequest(parentSpan, req)
// 	res, err := http.DefaultClient.Do(req)
// 	if err != nil {
// 		log.Println(err)
// 	}
// 	res.Body.Close()
// 	ht.Finish()
func TraceRequest(sp opentracing.Span, req *http.Request) (*http.Request, *Tracer) {
	ht := newTrace(sp)
	ctx := httptrace.WithClientTrace(req.Context(), ht.clientTrace())
	req = req.WithContext(context.WithValue(ctx, keyTracer, ht))
	return req, ht
}

type closeTracker struct {
	io.ReadCloser
	sp opentracing.Span
}

func (c closeTracker) Close() error {
	err := c.ReadCloser.Close()
	c.sp.LogEvent("Closed body")
	c.sp.Finish()
	return err
}

// RoundTrip implements the RoundTripper interface.
func (t *Transport) RoundTrip(req *http.Request) (*http.Response, error) {
	tracer, ok := req.Context().Value(keyTracer).(*Tracer)
	if !ok {
		return t.RoundTripper.RoundTrip(req)
	}

	tracer.start(req)

	ext.HTTPMethod.Set(tracer.sp, req.Method)
	ext.HTTPUrl.Set(tracer.sp, req.URL.String())

	carrier := opentracing.HTTPHeaderTextMapCarrier(req.Header)
	tracer.sp.Tracer().Inject(tracer.sp.Context(), opentracing.TextMap, carrier)
	resp, err := t.RoundTripper.RoundTrip(req)

	ext.HTTPStatusCode.Set(tracer.sp, uint16(resp.StatusCode))
	if err != nil && req.Method == "HEAD" {
		tracer.sp.Finish()
	} else {
		resp.Body = closeTracker{resp.Body, tracer.sp}
	}
	return resp, err
}

type Tracer struct {
	root   opentracing.Span
	parent opentracing.Span
	sp     opentracing.Span
}

func (h *Tracer) start(req *http.Request) opentracing.Span {
	var ctx opentracing.SpanContext
	if h.parent != nil {
		ctx = h.parent.Context()
	}
	h.sp = h.parent.Tracer().StartSpan("HTTP "+req.Method, opentracing.ChildOf(ctx))
	h.parent = h.sp
	ext.SpanKindRPCClient.Set(h.sp)
	ext.Component.Set(h.sp, "net/http")

	return h.sp
}

// Finish finishes the span of the traced request.
func (h *Tracer) Finish() {
	h.root.Finish()
}

// Span returns the span of the traced request.
func (h *Tracer) Span() opentracing.Span {
	return h.root
}

func newTrace(parent opentracing.Span) *Tracer {
	var ctx opentracing.SpanContext
	if parent != nil {
		ctx = parent.Context()
	}
	root := parent.Tracer().StartSpan("HTTP", opentracing.ChildOf(ctx))
	ext.SpanKindRPCClient.Set(root)
	return &Tracer{root, root, nil}
}

func (h *Tracer) clientTrace() *httptrace.ClientTrace {
	return &httptrace.ClientTrace{
		GetConn:              h.getConn,
		GotConn:              h.gotConn,
		PutIdleConn:          h.putIdleConn,
		GotFirstResponseByte: h.gotFirstResponseByte,
		Got100Continue:       h.got100Continue,
		DNSStart:             h.dnsStart,
		DNSDone:              h.dnsDone,
		ConnectStart:         h.connectStart,
		ConnectDone:          h.connectDone,
		WroteHeaders:         h.wroteHeaders,
		Wait100Continue:      h.wait100Continue,
		WroteRequest:         h.wroteRequest,
	}
}

func (h *Tracer) getConn(hostPort string) {
	ext.HTTPUrl.Set(h.sp, hostPort)
	h.sp.LogEvent("Get conn")
}

func (h *Tracer) gotConn(info httptrace.GotConnInfo) {
	h.sp.SetTag("net/http.reused", info.Reused)
	h.sp.SetTag("net/http.was_idle", info.WasIdle)
	h.sp.LogEvent("Got conn")
}

func (h *Tracer) putIdleConn(error) {
	h.sp.LogEvent("Put idle conn")
}

func (h *Tracer) gotFirstResponseByte() {
	h.sp.LogEvent("Got first response byte")
}

func (h *Tracer) got100Continue() {
	h.sp.LogEvent("Got 100 continue")
}

func (h *Tracer) dnsStart(info httptrace.DNSStartInfo) {
	h.sp.LogEventWithPayload("DNS start", info.Host)
}

func (h *Tracer) dnsDone(httptrace.DNSDoneInfo) {
	h.sp.LogEvent("DNS done")
}

func (h *Tracer) connectStart(network, addr string) {
	h.sp.LogEventWithPayload("Connect start", network+":"+addr)
}

func (h *Tracer) connectDone(network, addr string, err error) {
	h.sp.LogEventWithPayload("Connect start", network+":"+addr)
}

func (h *Tracer) wroteHeaders() {
	h.sp.LogEvent("Wrote headers")
}

func (h *Tracer) wait100Continue() {
	h.sp.LogEvent("Wait 100 continue")
}

func (h *Tracer) wroteRequest(info httptrace.WroteRequestInfo) {
	if info.Err != nil {
		ext.Error.Set(h.sp, true)
	}
	h.sp.LogEvent("Wrote request")
}
