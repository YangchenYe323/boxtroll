package bilibili

import (
	"context"
	"fmt"
	"io"
	"maps"
	"net/http"
)

// A wrapper for http.Client that handles passing along default headers and credentials
// to the bilibili APIs.
type Client struct {
	HttpClient     *http.Client
	DefaultHeaders http.Header
	Credential     *Credential
	WbiKeys        *WbiKeys
}

var DefaultClient = &Client{
	HttpClient: http.DefaultClient,
	DefaultHeaders: http.Header{
		"User-Agent": {"Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36"},
	},
	WbiKeys: DefaultWbiKeys,
}

func (c *Client) Login(credential *Credential) {
	c.Credential = credential
}

func (c *Client) Get(url string) (*http.Response, error) {
	req, err := c.NewRequestWithContext(context.Background(), http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	return c.Do(req)
}

func (c *Client) NewRequestWithContext(ctx context.Context, method, url string, body io.Reader) (*http.Request, error) {
	req, err := http.NewRequestWithContext(ctx, method, url, body)
	if err != nil {
		return nil, err
	}

	maps.Copy(req.Header, c.DefaultHeaders)
	if c.Credential != nil {
		req.Header.Add("COOKIE", fmt.Sprintf("SESSDATA=%s", c.Credential.SessionData))
		if c.Credential.Buvid3 != "" {
			req.Header.Add("COOKIE", fmt.Sprintf("buvid3=%s", c.Credential.Buvid3))
		}
		if c.Credential.BiliJct != "" {
			req.Header.Add("COOKIE", fmt.Sprintf("bili_jct=%s", c.Credential.BiliJct))
		}
	}

	return req, nil
}

func (c *Client) Do(req *http.Request) (*http.Response, error) {
	if c == nil {
		c = DefaultClient
	}

	return c.HttpClient.Do(req)
}
