package web

import (
	"bytes"
	"fmt"
	"log"
	"net/http"
	"path"
	"runtime"
	"strings"
	"sync"
)

// timeoutWriter Code borrowed from net/http/server.go of go1.18.3 linux/amd64
type timeoutWriter struct {
	originalResponseWriter http.ResponseWriter
	headers                http.Header
	bodyBytes              bytes.Buffer
	req                    *http.Request

	writeMutex        sync.Mutex
	writeError        error
	headersAreWritten bool
	httpStatus        int
}

func (tw *timeoutWriter) Header() http.Header {
	return tw.headers
}

func (tw *timeoutWriter) Write(p []byte) (int, error) {
	tw.writeMutex.Lock()
	defer tw.writeMutex.Unlock()
	if tw.writeError != nil {
		return 0, tw.writeError
	}
	if !tw.headersAreWritten {
		tw.writeHeaderLocked(http.StatusOK)
	}
	return tw.bodyBytes.Write(p)
}

func (tw *timeoutWriter) writeHeaderLocked(httpStatus int) {
	checkWriteHeaderCode(httpStatus)

	switch {
	case tw.writeError != nil:
		return
	case tw.headersAreWritten:
		if tw.req != nil {
			caller := relevantCaller()
			logf(tw.req, "http: superfluous response.WriteHeader call from %s (%s:%d)", caller.Function, path.Base(caller.File), caller.Line)
		}
	default:
		tw.headersAreWritten = true
		tw.httpStatus = httpStatus
	}
}

func (tw *timeoutWriter) WriteHeader(code int) {
	tw.writeMutex.Lock()
	defer tw.writeMutex.Unlock()
	tw.writeHeaderLocked(code)
}

func newTimeoutWriter(responseWriter http.ResponseWriter, requestObj *http.Request) *timeoutWriter {
	return &timeoutWriter{
		originalResponseWriter: responseWriter,
		headers:                make(http.Header),
		req:                    requestObj,
		httpStatus:             http.StatusOK,
	}
}

func checkWriteHeaderCode(code int) {
	// Issue 22880: require valid WriteHeader status codes.
	// For now we only enforce that it's three digits.
	// In the future we might block things over 599 (600 and above aren't defined
	// at https://httpwg.org/specs/rfc7231.html#status.codes)
	// and we might block under 200 (once we have more mature 1xx support).
	// But for now any three digits.
	//
	// We used to send "HTTP/1.1 000 0" on the wire in responses but there's
	// no equivalent bogus thing we can realistically send in HTTP/2,
	// so we'll consistently panic instead and help people find their bugs
	// early. (We can't return an error from WriteHeader even if we wanted to.)
	if code < 100 || code > 999 {
		panic(fmt.Sprintf("invalid WriteHeader code %v", code))
	}
}

// logf prints to the ErrorLog of the *Server associated with request r
// via ServerContextKey. If there's no associated server, or if ErrorLog
// is nil, logging is done via the log package's standard logger.
func logf(r *http.Request, format string, args ...interface{}) {
	s, _ := r.Context().Value(http.ServerContextKey).(*http.Server)
	if s != nil && s.ErrorLog != nil {
		s.ErrorLog.Printf(format, args...)
	} else {
		log.Printf(format, args...)
	}
}

// relevantCaller searches the call stack for the first function outside of net/http.
// The purpose of this function is to provide more helpful error messages.
func relevantCaller() runtime.Frame {
	pc := make([]uintptr, 16)
	n := runtime.Callers(1, pc)
	frames := runtime.CallersFrames(pc[:n])
	var frame runtime.Frame
	for {
		frame, more := frames.Next()
		if !strings.HasPrefix(frame.Function, "net/http.") {
			return frame
		}
		if !more {
			break
		}
	}
	return frame
}
