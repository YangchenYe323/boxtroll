package bilibili

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
)

type MessageStreamInfo struct {
	Token    string          `json:"token"`
	HostList []*LiveEndpoint `json:"host_list"`
}

type LiveEndpoint struct {
	Host    string `json:"host"`
	Port    int    `json:"port"`
	WssPort int    `json:"wss_port"`
	WsPort  int    `json:"ws_port"`
}

func GetMessageStreamInfo(ctx context.Context, liveRoomID int64) (*MessageStreamInfo, error) {
	return DefaultClient.GetMessageStreamInfo(ctx, liveRoomID)
}

func (c *Client) GetMessageStreamInfo(ctx context.Context, liveRoomID int64) (*MessageStreamInfo, error) {
	if c.Credential == nil {
		return nil, errors.New("credential is nil")
	}

	if err := c.WbiKeys.update(ctx, c, false); err != nil {
		return nil, err
	}

	req, err := c.NewRequestWithContext(
		ctx,
		http.MethodGet,
		fmt.Sprintf("https://api.live.bilibili.com/xlive/web-room/v1/index/getDanmuInfo?id=%d", liveRoomID),
		nil,
	)

	if err != nil {
		return nil, err
	}

	if err := c.WbiKeys.Sign(req.URL); err != nil {
		return nil, err
	}

	resp, err := c.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var n response[MessageStreamInfo]
	if err := json.Unmarshal(body, &n); err != nil {
		return nil, err
	}

	data, err := n.DataOrError()
	if err != nil {
		return nil, err
	}

	return data, nil
}
