// Package jsonapi interacts with HTTP JSON APIs. In OONI we use
// this code when accessing API like, e.g., the OONI collector.
package jsonapi

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"net/url"

	"github.com/ooni/probe-engine/log"
)

// Client is a client for a JSON API.
type Client struct {
	// BaseURL is the base URL of the API.
	BaseURL string

	// HTTPClient is the http client to use.
	HTTPClient *http.Client

	// Logger is the logger to use.
	Logger log.Logger

	// UserAgent is the user agent to use.
	UserAgent string
}

func (c *Client) makePostRequest(
	ctx context.Context, method, resourcePath string,
	query url.Values, body interface{},
) (*http.Request, error) {
	data, err := json.Marshal(body)
	if err != nil {
		return nil, err
	}
	c.Logger.Debugf("jsonapi: request body: %s", string(data))
	return c.makeRequest(ctx, method, resourcePath, query, bytes.NewReader(data))
}

func (c *Client) makeRequest(
	ctx context.Context, method, resourcePath string,
	query url.Values, body io.Reader,
) (*http.Request, error) {
	URL, err := url.Parse(c.BaseURL)
	if err != nil {
		return nil, err
	}
	URL.Path = resourcePath
	if query != nil {
		URL.RawQuery = query.Encode()
	}
	c.Logger.Debugf("jsonapi: method: %s", method)
	c.Logger.Debugf("jsonapi: URL: %s", URL.String())
	request, err := http.NewRequest(method, URL.String(), body)
	if err != nil {
		return nil, err
	}
	if body != nil {
		request.Header.Set("Content-Type", "application/json")
	}
	request.Header.Set("User-Agent", c.UserAgent)
	return request.WithContext(ctx), nil
}

func (c *Client) do(request *http.Request, output interface{}) error {
	response, err := c.HTTPClient.Do(request)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	if response.StatusCode >= 400 {
		return fmt.Errorf("Request failed: %s", response.Status)
	}
	data, err := ioutil.ReadAll(response.Body)
	if err != nil {
		return err
	}
	c.Logger.Debugf("jsonapi: response body: %s", string(data))
	return json.Unmarshal(data, output)
}

// Read reads the JSON resource at resourcePath and unmarshals the
// results into output. The request is bounded by the lifetime of the
// context passed as argument. Returns the error that occurred.
func (c *Client) Read(
	ctx context.Context, resourcePath string, output interface{},
) error {
	request, err := c.makeRequest(ctx, "GET", resourcePath, nil, nil)
	if err != nil {
		return err
	}
	return c.do(request, output)
}

// ReadWithQuery is like Read but also has a query.
func (c *Client) ReadWithQuery(
	ctx context.Context, resourcePath string,
	query url.Values, output interface{},
) error {
	request, err := c.makeRequest(ctx, "GET", resourcePath, query, nil)
	if err != nil {
		return err
	}
	return c.do(request, output)
}

// Create creates a JSON subresource of the resource at resourcePath
// using the JSON document at input and returning the result into the
// JSON document at output. The request is bounded by the context's
// lifetime. Returns the error that occurred.
func (c *Client) Create(
	ctx context.Context, resourcePath string, input, output interface{},
) error {
	request, err := c.makePostRequest(ctx, "POST", resourcePath, nil, input)
	if err != nil {
		return err
	}
	return c.do(request, output)
}
