package api

import (
	"bytes"
	"encoding/json"
	"encoding/xml"
	"mime"
	"net/http"
	"strings"
)

const problemDetailsContentType = "application/problem+json"

type problemDetails struct {
	Type     string  `json:"type"`
	Title    string  `json:"title"`
	Detail   string  `json:"detail"`
	Status   int     `json:"status"`
	Instance *string `json:"instance,omitempty"`
}

type goaErrorResponse struct {
	XMLName   xml.Name `json:"-" xml:"error"`
	Name      string   `json:"name" xml:"name"`
	ID        string   `json:"id" xml:"id"`
	Message   string   `json:"message" xml:"message"`
	Temporary bool     `json:"temporary" xml:"temporary"`
	Timeout   bool     `json:"timeout" xml:"timeout"`
	Fault     bool     `json:"fault" xml:"fault"`
}

type bufferedResponseWriter struct {
	response http.ResponseWriter
	header   http.Header
	body     bytes.Buffer
	status   int
	written  bool
}

func problemDetailsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		buffer := newBufferedResponseWriter(w)
		next.ServeHTTP(buffer, r)

		if problem, name, ok := problemDetailsFromGoaError(buffer); ok {
			copyHeaders(w.Header(), buffer.Header())
			w.Header().Set("Content-Type", problemDetailsContentType)
			w.Header().Del("Content-Length")
			w.Header().Set("goa-error", name)
			w.WriteHeader(problem.Status)
			_ = json.NewEncoder(w).Encode(problem)
			return
		}

		buffer.Flush()
	})
}

func newBufferedResponseWriter(w http.ResponseWriter) *bufferedResponseWriter {
	return &bufferedResponseWriter{
		response: w,
		header:   make(http.Header),
		status:   http.StatusOK,
	}
}

func (w *bufferedResponseWriter) Header() http.Header {
	return w.header
}

func (w *bufferedResponseWriter) WriteHeader(status int) {
	if w.written {
		return
	}
	w.status = status
	w.written = true
}

func (w *bufferedResponseWriter) Write(body []byte) (int, error) {
	if !w.written {
		w.WriteHeader(http.StatusOK)
	}
	return w.body.Write(body)
}

func (w *bufferedResponseWriter) Flush() {
	copyHeaders(w.response.Header(), w.Header())
	if w.written {
		w.response.WriteHeader(w.status)
	}
	_, _ = w.body.WriteTo(w.response)
}

func problemDetailsFromGoaError(w *bufferedResponseWriter) (*problemDetails, string, bool) {
	if w.status < http.StatusBadRequest {
		return nil, "", false
	}

	// Goa decodes and validates requests before calling service methods. Errors
	// that are not explicit Goa method errors use Goa's default ErrorResponse
	// envelope, even though the design documents RFC 9457 ProblemDetails. Keep
	// Goa boundary validation for the generated schema, then normalize that
	// fallback envelope at the HTTP boundary so clients see one error shape.
	goaError, ok := decodeGoaErrorResponse(w.Header().Get("Content-Type"), w.body.Bytes())
	if !ok {
		return nil, "", false
	}

	title := http.StatusText(w.status)
	if title == "" {
		title = "Error"
	}

	return &problemDetails{
		Type:   "about:blank",
		Title:  title,
		Detail: goaError.Message,
		Status: w.status,
	}, goaError.Name, true
}

func decodeGoaErrorResponse(contentType string, body []byte) (*goaErrorResponse, bool) {
	body = bytes.TrimSpace(body)
	if len(body) == 0 {
		return nil, false
	}

	var mediaType string
	if contentType != "" {
		parsed, _, err := mime.ParseMediaType(contentType)
		if err == nil {
			mediaType = parsed
		}
	}

	var err error
	var goaError goaErrorResponse
	switch {
	case mediaType == "", mediaType == "application/json", strings.HasSuffix(mediaType, "+json"):
		err = json.Unmarshal(body, &goaError)
	case mediaType == "application/xml", strings.HasSuffix(mediaType, "+xml"):
		err = xml.Unmarshal(body, &goaError)
	default:
		return nil, false
	}
	if err != nil || goaError.Name == "" || goaError.ID == "" || goaError.Message == "" {
		return nil, false
	}

	return &goaError, true
}

func copyHeaders(dst, src http.Header) {
	for key, values := range src {
		dst.Del(key)
		for _, value := range values {
			dst.Add(key, value)
		}
	}
}
