package bilibili

import (
	"context"
	"encoding/json"
	"net/http"
)

type Buvid struct {
	B3 string `json:"b_3"`
	B4 string `json:"b_4"`
}

func GetBuvid(ctx context.Context) (*Buvid, error) {
	return DefaultClient.GetBuvid(ctx)
}

func (c *Client) GetBuvid(ctx context.Context) (*Buvid, error) {
	req, err := c.NewRequestWithContext(ctx, http.MethodGet, "https://api.bilibili.com/x/frontend/finger/spi", nil)
	if err != nil {
		return nil, err
	}

	resp, err := c.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var response response[Buvid]
	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return nil, err
	}

	data, err := response.DataOrError()
	if err != nil {
		return nil, err
	}

	return data, nil
}
