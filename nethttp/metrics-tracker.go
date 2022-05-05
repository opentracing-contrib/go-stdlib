// +build go1.8

package nethttp

import (
	"io"
	"net/http"
)

type metricsTracker struct {
	http.ResponseWriter
	status int
	size   int
}

func (w *metricsTracker) WriteHeader(status int) {
	w.status = status
	w.ResponseWriter.WriteHeader(status)
}

func (w *metricsTracker) Write(b []byte) (int, error) {
	size, err := w.ResponseWriter.Write(b)
	w.size += size
	return size, err
}

func (w *metricsTracker) WriteString(s string) (int, error) {
	size, err := io.WriteString(w.ResponseWriter, s)
	w.size += size
	return size, err
}

// wrappedResponseWriter returns a wrapped version of the original
// ResponseWriter and only implements the same combination of additional
// interfaces as the original.  This implementation is based on
// https://github.com/felixge/httpsnoop.
func (w *metricsTracker) wrappedResponseWriter() http.ResponseWriter {
	var (
		hj, i0 = w.ResponseWriter.(http.Hijacker)
		cn, i1 = w.ResponseWriter.(http.CloseNotifier)
		pu, i2 = w.ResponseWriter.(http.Pusher)
		fl, i3 = w.ResponseWriter.(http.Flusher)
		rf, i4 = w.ResponseWriter.(io.ReaderFrom)
	)

	switch {
	case !i0 && !i1 && !i2 && !i3 && !i4:
		return struct {
			http.ResponseWriter
			io.StringWriter
		}{w, w}
	case !i0 && !i1 && !i2 && !i3 && i4:
		return struct {
			http.ResponseWriter
			io.StringWriter
			io.ReaderFrom
		}{w, w, rf}
	case !i0 && !i1 && !i2 && i3 && !i4:
		return struct {
			http.ResponseWriter
			io.StringWriter
			http.Flusher
		}{w, w, fl}
	case !i0 && !i1 && !i2 && i3 && i4:
		return struct {
			http.ResponseWriter
			io.StringWriter
			http.Flusher
			io.ReaderFrom
		}{w, w, fl, rf}
	case !i0 && !i1 && i2 && !i3 && !i4:
		return struct {
			http.ResponseWriter
			io.StringWriter
			http.Pusher
		}{w, w, pu}
	case !i0 && !i1 && i2 && !i3 && i4:
		return struct {
			http.ResponseWriter
			io.StringWriter
			http.Pusher
			io.ReaderFrom
		}{w, w, pu, rf}
	case !i0 && !i1 && i2 && i3 && !i4:
		return struct {
			http.ResponseWriter
			io.StringWriter
			http.Pusher
			http.Flusher
		}{w, w, pu, fl}
	case !i0 && !i1 && i2 && i3 && i4:
		return struct {
			http.ResponseWriter
			io.StringWriter
			http.Pusher
			http.Flusher
			io.ReaderFrom
		}{w, w, pu, fl, rf}
	case !i0 && i1 && !i2 && !i3 && !i4:
		return struct {
			http.ResponseWriter
			io.StringWriter
			http.CloseNotifier
		}{w, w, cn}
	case !i0 && i1 && !i2 && !i3 && i4:
		return struct {
			http.ResponseWriter
			io.StringWriter
			http.CloseNotifier
			io.ReaderFrom
		}{w, w, cn, rf}
	case !i0 && i1 && !i2 && i3 && !i4:
		return struct {
			http.ResponseWriter
			io.StringWriter
			http.CloseNotifier
			http.Flusher
		}{w, w, cn, fl}
	case !i0 && i1 && !i2 && i3 && i4:
		return struct {
			http.ResponseWriter
			io.StringWriter
			http.CloseNotifier
			http.Flusher
			io.ReaderFrom
		}{w, w, cn, fl, rf}
	case !i0 && i1 && i2 && !i3 && !i4:
		return struct {
			http.ResponseWriter
			io.StringWriter
			http.CloseNotifier
			http.Pusher
		}{w, w, cn, pu}
	case !i0 && i1 && i2 && !i3 && i4:
		return struct {
			http.ResponseWriter
			io.StringWriter
			http.CloseNotifier
			http.Pusher
			io.ReaderFrom
		}{w, w, cn, pu, rf}
	case !i0 && i1 && i2 && i3 && !i4:
		return struct {
			http.ResponseWriter
			io.StringWriter
			http.CloseNotifier
			http.Pusher
			http.Flusher
		}{w, w, cn, pu, fl}
	case !i0 && i1 && i2 && i3 && i4:
		return struct {
			http.ResponseWriter
			io.StringWriter
			http.CloseNotifier
			http.Pusher
			http.Flusher
			io.ReaderFrom
		}{w, w, cn, pu, fl, rf}
	case i0 && !i1 && !i2 && !i3 && !i4:
		return struct {
			http.ResponseWriter
			io.StringWriter
			http.Hijacker
		}{w, w, hj}
	case i0 && !i1 && !i2 && !i3 && i4:
		return struct {
			http.ResponseWriter
			io.StringWriter
			http.Hijacker
			io.ReaderFrom
		}{w, w, hj, rf}
	case i0 && !i1 && !i2 && i3 && !i4:
		return struct {
			http.ResponseWriter
			io.StringWriter
			http.Hijacker
			http.Flusher
		}{w, w, hj, fl}
	case i0 && !i1 && !i2 && i3 && i4:
		return struct {
			http.ResponseWriter
			io.StringWriter
			http.Hijacker
			http.Flusher
			io.ReaderFrom
		}{w, w, hj, fl, rf}
	case i0 && !i1 && i2 && !i3 && !i4:
		return struct {
			http.ResponseWriter
			io.StringWriter
			http.Hijacker
			http.Pusher
		}{w, w, hj, pu}
	case i0 && !i1 && i2 && !i3 && i4:
		return struct {
			http.ResponseWriter
			io.StringWriter
			http.Hijacker
			http.Pusher
			io.ReaderFrom
		}{w, w, hj, pu, rf}
	case i0 && !i1 && i2 && i3 && !i4:
		return struct {
			http.ResponseWriter
			io.StringWriter
			http.Hijacker
			http.Pusher
			http.Flusher
		}{w, w, hj, pu, fl}
	case i0 && !i1 && i2 && i3 && i4:
		return struct {
			http.ResponseWriter
			io.StringWriter
			http.Hijacker
			http.Pusher
			http.Flusher
			io.ReaderFrom
		}{w, w, hj, pu, fl, rf}
	case i0 && i1 && !i2 && !i3 && !i4:
		return struct {
			http.ResponseWriter
			io.StringWriter
			http.Hijacker
			http.CloseNotifier
		}{w, w, hj, cn}
	case i0 && i1 && !i2 && !i3 && i4:
		return struct {
			http.ResponseWriter
			io.StringWriter
			http.Hijacker
			http.CloseNotifier
			io.ReaderFrom
		}{w, w, hj, cn, rf}
	case i0 && i1 && !i2 && i3 && !i4:
		return struct {
			http.ResponseWriter
			io.StringWriter
			http.Hijacker
			http.CloseNotifier
			http.Flusher
		}{w, w, hj, cn, fl}
	case i0 && i1 && !i2 && i3 && i4:
		return struct {
			http.ResponseWriter
			io.StringWriter
			http.Hijacker
			http.CloseNotifier
			http.Flusher
			io.ReaderFrom
		}{w, w, hj, cn, fl, rf}
	case i0 && i1 && i2 && !i3 && !i4:
		return struct {
			http.ResponseWriter
			io.StringWriter
			http.Hijacker
			http.CloseNotifier
			http.Pusher
		}{w, w, hj, cn, pu}
	case i0 && i1 && i2 && !i3 && i4:
		return struct {
			http.ResponseWriter
			io.StringWriter
			http.Hijacker
			http.CloseNotifier
			http.Pusher
			io.ReaderFrom
		}{w, w, hj, cn, pu, rf}
	case i0 && i1 && i2 && i3 && !i4:
		return struct {
			http.ResponseWriter
			io.StringWriter
			http.Hijacker
			http.CloseNotifier
			http.Pusher
			http.Flusher
		}{w, w, hj, cn, pu, fl}
	case i0 && i1 && i2 && i3 && i4:
		return struct {
			http.ResponseWriter
			io.StringWriter
			http.Hijacker
			http.CloseNotifier
			http.Pusher
			http.Flusher
			io.ReaderFrom
		}{w, w, hj, cn, pu, fl, rf}
	default:
		return struct {
			http.ResponseWriter
			io.StringWriter
		}{w, w}
	}
}
