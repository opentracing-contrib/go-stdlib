//go:build go1.7
// +build go1.7

package nethttp

import (
	"net/http"
	"net/url"

	"github.com/opentracing/opentracing-go"
	"github.com/opentracing/opentracing-go/ext"
)

var responseSizeKey = "http.response_size"

type mwOptions struct {
	opNameFunc       func(r *http.Request) string
	spanFilter       func(r *http.Request) bool
	startSpanOptions func(r *http.Request) []opentracing.StartSpanOption
	spanObserver     func(span opentracing.Span, r *http.Request)
	urlTagFunc       func(u *url.URL) string
	componentName    string
}

// MWOption controls the behavior of the Middleware.
type MWOption func(*mwOptions)

// OperationNameFunc returns a MWOption that uses given function f
// to generate operation name for each server-side span.
func OperationNameFunc(f func(r *http.Request) string) MWOption {
	return func(options *mwOptions) {
		options.opNameFunc = f
	}
}

// MWComponentName returns a MWOption that sets the component name
// for the server-side span.
func MWComponentName(componentName string) MWOption {
	return func(options *mwOptions) {
		options.componentName = componentName
	}
}

// MWSpanFilter returns a MWOption that filters requests from creating a span
// for the server-side span.
// Span won't be created if it returns false.
func MWSpanFilter(f func(r *http.Request) bool) MWOption {
	return func(options *mwOptions) {
		options.spanFilter = f
	}
}

// MWStartSpanOptions returns a MWOption that creates options for starting a span.
// Middleware first applies default StartSpanOptions, followed by the ones supplied by the user.
func MWStartSpanOptions(f func(r *http.Request) []opentracing.StartSpanOption) MWOption {
	return func(options *mwOptions) {
		options.startSpanOptions = f
	}
}

// MWSpanObserver returns a MWOption that observe the span
// for the server-side span.
func MWSpanObserver(f func(span opentracing.Span, r *http.Request)) MWOption {
	return func(options *mwOptions) {
		options.spanObserver = f
	}
}

// MWURLTagFunc returns a MWOption that uses given function f
// to set the span's http.url tag. Can be used to change the default
// http.url tag, eg to redact sensitive information.
func MWURLTagFunc(f func(u *url.URL) string) MWOption {
	return func(options *mwOptions) {
		options.urlTagFunc = f
	}
}

// Middleware wraps an http.Handler and traces incoming requests.
// Additionally, it adds the span to the request's context.
//
// By default, the operation name of the spans is set to "HTTP {method}".
// This can be overridden with options.
//
// Example:
//
//	http.ListenAndServe("localhost:80", nethttp.Middleware(tracer, http.DefaultServeMux))
//
// The options allow fine tuning the behavior of the middleware.
//
// Example:
//
//	  mw := nethttp.Middleware(
//	     tracer,
//	     http.DefaultServeMux,
//	     nethttp.OperationNameFunc(func(r *http.Request) string {
//		        return "HTTP " + r.Method + ":/api/customers"
//	     }),
//	     nethttp.MWSpanObserver(func(sp opentracing.Span, r *http.Request) {
//				sp.SetTag("http.uri", r.URL.EscapedPath())
//			}),
//	  )
func Middleware(tr opentracing.Tracer, h http.Handler, options ...MWOption) http.Handler {
	return MiddlewareFunc(tr, h.ServeHTTP, options...)
}

// MiddlewareFunc wraps an http.HandlerFunc and traces incoming requests.
// It behaves identically to the Middleware function above.
//
// Example:
//
//	http.ListenAndServe("localhost:80", nethttp.MiddlewareFunc(tracer, MyHandler))
func MiddlewareFunc(tr opentracing.Tracer, h http.HandlerFunc, options ...MWOption) http.HandlerFunc {
	opts := mwOptions{
		opNameFunc: func(r *http.Request) string {
			return "HTTP " + r.Method
		},
		spanFilter:       func(r *http.Request) bool { return true },
		startSpanOptions: func(r *http.Request) []opentracing.StartSpanOption { return nil },
		spanObserver:     func(span opentracing.Span, r *http.Request) {},
		urlTagFunc: func(u *url.URL) string {
			return u.String()
		},
	}
	for _, opt := range options {
		opt(&opts)
	}
	// set component name, use "net/http" if caller does not specify
	componentName := opts.componentName
	if componentName == "" {
		componentName = defaultComponentName
	}

	fn := func(w http.ResponseWriter, r *http.Request) {
		if !opts.spanFilter(r) {
			h(w, r)
			return
		}
		ctx, _ := tr.Extract(opentracing.HTTPHeaders, opentracing.HTTPHeadersCarrier(r.Header))
		startSpanOptions := collectStartSpanOptions(ctx, r, opts)
		sp := tr.StartSpan(opts.opNameFunc(r), startSpanOptions...)
		ext.HTTPMethod.Set(sp, r.Method)
		ext.HTTPUrl.Set(sp, opts.urlTagFunc(r.URL))
		ext.Component.Set(sp, componentName)
		opts.spanObserver(sp, r)

		mt := &metricsTracker{ResponseWriter: w}
		r = r.WithContext(opentracing.ContextWithSpan(r.Context(), sp))

		defer func() {
			panicErr := recover()
			didPanic := panicErr != nil

			if mt.status == 0 && !didPanic {
				// Standard behavior of http.Server is to assume status code 200 if one was not written by a handler that returned successfully.
				// https://github.com/golang/go/blob/fca286bed3ed0e12336532cc711875ae5b3cb02a/src/net/http/server.go#L120
				mt.status = 200
			}
			if mt.status > 0 {
				ext.HTTPStatusCode.Set(sp, uint16(mt.status)) //nolint:gosec // can't have integer overflow with status code
			}
			if mt.size > 0 {
				sp.SetTag(responseSizeKey, mt.size)
			}
			if mt.status >= http.StatusInternalServerError || didPanic {
				ext.Error.Set(sp, true)
			}
			sp.Finish()

			if didPanic {
				panic(panicErr)
			}
		}()

		h(mt.wrappedResponseWriter(), r)
	}
	return http.HandlerFunc(fn)
}

func collectStartSpanOptions(ctx opentracing.SpanContext, r *http.Request, opts mwOptions) []opentracing.StartSpanOption {
	mwStartSpanOptions := opts.startSpanOptions(r)

	startSpanOptions := make([]opentracing.StartSpanOption, 0, len(mwStartSpanOptions)+1)
	startSpanOptions = append(startSpanOptions, ext.RPCServerOption(ctx))
	startSpanOptions = append(startSpanOptions, mwStartSpanOptions...)

	return startSpanOptions
}
