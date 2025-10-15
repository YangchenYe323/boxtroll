package bilibili

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
)

type QRCode struct {
	URL string `json:"url"`
	Key string `json:"qrcode_key"`
}

func GetLoginQRCode(ctx context.Context) (*QRCode, error) {
	return DefaultClient.GetLoginQRCode(ctx)
}

func (c *Client) GetLoginQRCode(ctx context.Context) (*QRCode, error) {
	req, err := c.NewRequestWithContext(ctx, http.MethodGet, "https://passport.bilibili.com/x/passport-login/web/qrcode/generate", nil)
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

	var n response[QRCode]
	if err := json.Unmarshal(body, &n); err != nil {
		return nil, err
	}

	data, err := n.DataOrError()
	if err != nil {
		return nil, err
	}

	return data, nil
}

type LoginStatus int

const (
	// 已登录
	LoginStatusSuccess LoginStatus = 0
	// 二维码已失效
	LoginStatusCodeExpired LoginStatus = 86038
	// 二维码已扫码未确认
	LoginStatusCodeScanned LoginStatus = 86090
	// 未扫码
	LoginStatusCodeUnscanned LoginStatus = 86101
)

type LoginPollResult struct {
	Code    LoginStatus `json:"code"`
	Message string      `json:"message"`
}

func PollLogin(ctx context.Context, key string) (*LoginPollResult, *Credential, error) {
	return DefaultClient.PollLogin(ctx, key)
}

func (c *Client) PollLogin(ctx context.Context, key string) (*LoginPollResult, *Credential, error) {
	req, err := c.NewRequestWithContext(
		ctx,
		http.MethodGet,
		fmt.Sprintf("https://passport.bilibili.com/x/passport-login/web/qrcode/poll?qrcode_key=%s", key),
		nil,
	)
	if err != nil {
		return nil, nil, err
	}

	resp, err := c.Do(req)
	if err != nil {
		return nil, nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, nil, err
	}

	var n response[LoginPollResult]
	if err := json.Unmarshal(body, &n); err != nil {
		return nil, nil, err
	}

	data, err := n.DataOrError()
	if err != nil {
		return nil, nil, err
	}

	var cred *Credential
	if data.Code == 0 {
		cred, err = credentialFromSetCookie(resp.Header.Values("Set-Cookie"))
		if err != nil {
			return nil, nil, err
		}
	}

	return data, cred, nil
}

func credentialFromSetCookie(setCookies []string) (*Credential, error) {
	var c Credential

	for _, cookie := range setCookies {
		if after, ok := strings.CutPrefix(cookie, "SESSDATA="); ok {
			c.SessionData = strings.Split(after, ";")[0]
		}
		if after, ok := strings.CutPrefix(cookie, "bili_jct="); ok {
			c.BiliJct = strings.Split(after, ";")[0]
		}
		if after, ok := strings.CutPrefix(cookie, "DedeUserID="); ok {
			c.DedeUserID = strings.Split(after, ";")[0]
		}
		if after, ok := strings.CutPrefix(cookie, "DedeUserID__ckMd5="); ok {
			c.DedeUserIDCkMd5 = strings.Split(after, ";")[0]
		}
	}

	if c.SessionData == "" {
		return nil, fmt.Errorf("cookie中未找到SESSDATA: %v", setCookies)
	}

	if c.BiliJct == "" {
		return nil, fmt.Errorf("cookie中未找到bili_jct: %v", setCookies)
	}

	return &c, nil
}
