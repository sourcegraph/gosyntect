package gosyntect

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/pkg/errors"
)

// Query represents a code highlighting query to the syntect_server.
type Query struct {
	// Extension is the file extension of the code.
	//
	// See https://github.com/sourcegraph/syntect_server#supported-file-extensions
	Extension string `json:"extension"`

	// Theme is the color theme to use for highlighting.
	//
	// See https://github.com/sourcegraph/syntect_server#embedded-themes
	Theme string `json:"theme"`

	// Code is the literal code to highlight.
	Code string `json:"code"`
}

// Response represents a response to a code highlighting query.
type Response struct {
	// Data is the actual highlighted HTML version of Query.Code.
	Data string
}

// Error is an error returned from the syntect_server.
type Error string

func (e Error) Error() string {
	return string(e)
}

type response struct {
	Data  string `json:"data"`
	Error string `json:"error"`
}

// Client represents a client connection to a syntect_server.
type Client struct {
	// Client can be overridden to be something other than http.DefaultClient.
	Client *http.Client

	syntectServer string
}

// Highlight performs a query to highlight some code.
func (c *Client) Highlight(q *Query) (*Response, error) {
	jsonQuery, err := json.Marshal(q)
	if err != nil {
		return nil, errors.Wrap(err, "encoding query")
	}
	resp, err := c.Client.Post(c.url("/"), "application/json", bytes.NewReader(jsonQuery))
	if err != nil {
		return nil, errors.Wrap(err, fmt.Sprintf("making request to %s", c.url("/")))
	}
	defer resp.Body.Close()
	var r response
	if err := json.NewDecoder(resp.Body).Decode(&r); err != nil {
		return nil, errors.Wrap(err, fmt.Sprintf("decoding JSON response from %s", c.url("/")))
	}
	if r.Error != "" {
		return nil, errors.Wrap(Error(r.Error), c.syntectServer)
	}
	return &Response{
		Data: r.Data,
	}, nil
}

func (c *Client) url(path string) string {
	return c.syntectServer + path
}

// New returns a client connection to a syntect_server.
func New(syntectServer string) *Client {
	return &Client{
		Client:        http.DefaultClient,
		syntectServer: strings.TrimSuffix(syntectServer, "/"),
	}
}
