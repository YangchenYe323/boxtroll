package bilibili

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

type MyInfo struct {
	MID  int64  `json:"mid"`
	Name string `json:"name"`
	Face string `json:"face"`
}

func GetMyInfo(ctx context.Context) (*MyInfo, error) {
	return DefaultClient.GetMyInfo(ctx)
}

// Get the user info from the currently logged in credential
func (c *Client) GetMyInfo(ctx context.Context) (*MyInfo, error) {
	if c.Credential == nil {
		return nil, ErrNeedLogin
	}

	if err := c.WbiKeys.update(ctx, c, false); err != nil {
		return nil, err
	}

	req, err := c.NewRequestWithContext(
		ctx,
		http.MethodGet,
		"https://api.bilibili.com/x/space/myinfo",
		nil,
	)
	if err != nil {
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

	var n response[MyInfo]
	if err := json.Unmarshal(body, &n); err != nil {
		return nil, err
	}

	data, err := n.DataOrError()
	if err != nil {
		return nil, err
	}

	return data, nil
}

type UserInfo struct {
	MID  int64  `json:"mid"`
	Name string `json:"name"`
	Face string `json:"face"`
}

func GetUserInfo(ctx context.Context, uid int64) (*UserInfo, error) {
	return DefaultClient.GetUserInfo(ctx, uid)
}

func (c *Client) GetUserInfo(ctx context.Context, uid int64) (*UserInfo, error) {
	if c.Credential == nil {
		return nil, ErrNeedLogin
	}

	if err := c.WbiKeys.update(ctx, c, false); err != nil {
		return nil, err
	}

	req, err := c.NewRequestWithContext(
		ctx,
		http.MethodGet,
		fmt.Sprintf("https://api.bilibili.com/x/space/wbi/acc/info?mid=%d", uid),
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

	var n response[UserInfo]
	if err := json.Unmarshal(body, &n); err != nil {
		return nil, err
	}

	data, err := n.DataOrError()
	if err != nil {
		return nil, err
	}

	return data, nil
}
