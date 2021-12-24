package gosyntect

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/opentracing-contrib/go-stdlib/nethttp"
	opentracing "github.com/opentracing/opentracing-go"
	"github.com/pkg/errors"
)

// Query represents a code highlighting query to the syntect_server.
type Query struct {
	// Extension is deprecated: use Filepath instead.
	Extension string `json:"extension"`

	// Filepath is the file path of the code. It can be the full file path, or
	// just the name and extension.
	//
	// See: https://github.com/sourcegraph/syntect_server#supported-file-extensions
	Filepath string `json:"filepath"`

	// Theme is the color theme to use for highlighting.
	// If CSS is true, theme is ignored.
	//
	// See https://github.com/sourcegraph/syntect_server#embedded-themes
	Theme string `json:"theme"`

	// Code is the literal code to highlight.
	Code string `json:"code"`

	// CSS causes results to be returned in HTML table format with CSS class
	// names annotating the spans rather than inline styles.
	CSS bool `json:"css"`

	// LineLengthLimit is the maximum length of line that will be highlighted if set.
	// Defaults to no max if zero.
	// If CSS is false, LineLengthLimit is ignored.
	LineLengthLimit int `json:"line_length_limit,omitempty"`

	// StabilizeTimeout, if non-zero, overrides the default syntect_server
	// http-server-stabilizer timeout of 10s. This is most useful when a user
	// is requesting to highlight a very large file and is willing to wait
	// longer, but it is important this not _always_ be a long duration because
	// the worker's threads could get stuck at 100% CPU for this amount of
	// time if the user's request ends up being a problematic one.
	StabilizeTimeout time.Duration `json:"-"`

	// Tracer, if not nil, will be used to record opentracing spans associated with the query.
	Tracer opentracing.Tracer
}

// Response represents a response to a code highlighting query.
type Response struct {
	// Data is the actual highlighted HTML version of Query.Code.
	Data string

	// Plaintext indicates whether or not a syntax could not be found for the
	// file and instead it was rendered as plain text.
	Plaintext bool

	// Reports the time syntect took purely for highlighting
	TimeNanos int64
}

var (
	// ErrInvalidTheme is returned when the Query.Theme is not a valid theme.
	ErrInvalidTheme = errors.New("invalid theme")

	// ErrRequestTooLarge is returned when the request is too large for syntect_server to handle (e.g. file is too large to highlight).
	ErrRequestTooLarge = errors.New("request too large")

	// ErrPanic occurs when syntect_server panics while highlighting code. This
	// most often occurs when Syntect does not support e.g. an obscure or
	// relatively unused sublime-syntax feature and as a result panics.
	ErrPanic = errors.New("syntect panic while highlighting")

	// ErrHSSWorkerTimeout occurs when syntect_server's wrapper,
	// http-server-stabilizer notices syntect_server is taking too long to
	// serve a request, has most likely gotten stuck, and as such has been
	// restarted. This occurs rarely on certain files syntect_server cannot yet
	// handle for some reason.
	ErrHSSWorkerTimeout = errors.New("HSS worker timeout while serving request")
)

type response struct {
	// Successful response fields.
	Data      string `json:"data"`
	Plaintext bool   `json:"plaintext"`
	TimeNanos int64  `json:"time_ns"`

	// Error response fields.
	Error string `json:"error"`
	Code  string `json:"code"`
}

// Client represents a client connection to a syntect_server.
type Client struct {
	syntectServer string
}

var client = &http.Client{Transport: &nethttp.Transport{}}

// Highlight performs a query to highlight some code.
func (c *Client) Highlight(ctx context.Context, q *Query) (*Response, error) {
	// Build the request.
	jsonQuery, err := json.Marshal(q)
	if err != nil {
		return nil, errors.Wrap(err, "encoding query")
	}
	req, err := http.NewRequest("POST", c.url("/"), bytes.NewReader(jsonQuery))
	if err != nil {
		return nil, errors.Wrap(err, "building request")
	}
	req.Header.Set("Content-Type", "application/json")
	if q.StabilizeTimeout != 0 {
		req.Header.Set("X-Stabilize-Timeout", q.StabilizeTimeout.String())
	}

	// Add tracing to the request.
	tracer := q.Tracer
	if tracer == nil {
		tracer = opentracing.NoopTracer{}
	}
	req = req.WithContext(ctx)
	req, ht := nethttp.TraceRequest(tracer, req,
		nethttp.OperationName("Highlight"),
		nethttp.ClientTrace(false))
	defer ht.Finish()

	// Perform the request.
	resp, err := client.Do(req)
	if err != nil {
		return nil, errors.Wrap(err, fmt.Sprintf("making request to %s", c.url("/")))
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusBadRequest {
		return nil, ErrRequestTooLarge
	}

	// Can only call ht.Span() after the request has been executed, so add our span tags in now.
	ht.Span().SetTag("Filepath", q.Filepath)
	ht.Span().SetTag("Theme", q.Theme)
	ht.Span().SetTag("CSS", q.CSS)

	// Decode the response.
	var r response
	if err := json.NewDecoder(resp.Body).Decode(&r); err != nil {
		return nil, errors.Wrap(err, fmt.Sprintf("decoding JSON response from %s", c.url("/")))
	}
	if r.Error != "" {
		var err error
		switch r.Code {
		case "invalid_theme":
			err = ErrInvalidTheme
		case "resource_not_found":
			// resource_not_found is returned in the event of a 404, indicating a bug
			// in gosyntect.
			err = errors.New("gosyntect internal error: resource_not_found")
		case "panic":
			err = ErrPanic
		case "hss_worker_timeout":
			err = ErrHSSWorkerTimeout
		default:
			err = fmt.Errorf("unknown error=%q code=%q", r.Error, r.Code)
		}
		return nil, errors.Wrap(err, c.syntectServer)
	}
	return &Response{
		Data:      r.Data,
		Plaintext: r.Plaintext,
		TimeNanos: r.TimeNanos,
	}, nil
}

func (c *Client) url(path string) string {
	return c.syntectServer + path
}

// New returns a client connection to a syntect_server.
func New(syntectServer string) *Client {
	return &Client{
		syntectServer: strings.TrimSuffix(syntectServer, "/"),
	}
}
