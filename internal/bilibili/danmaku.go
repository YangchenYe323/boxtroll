package bilibili

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

const (
	// 提督和总督有更长的弹幕长度，暂时先不考虑
	MAX_DANMAKU_MSG_LEN = 20
)

type DanmakuConfig struct {
	Groups []*DanmakuColorGroup `json:"group"`
	Modes  []*DanmakuMode       `json:"mode"`
}

type DanmakuColorGroup struct {
	Name   string          `json:"name"`
	Sort   int64           `json:"sort"`
	Colors []*DanmakuColor `json:"color"`
}

type DanmakuColor struct {
	Name     string `json:"name"`
	Color    string `json:"color"`
	ColorHex string `json:"color_hex"`
	Status   int64  `json:"status"`
	Weight   int64  `json:"weight"`
	ColorId  int64  `json:"color_id"`
	Origin   int64  `json:"origin"`
}

type DanmakuMode struct {
	Name   string `json:"name"`
	Mode   int64  `json:"mode"`
	Type   string `json:"type"`
	Status int64  `json:"status"`
}

func GetDanmakuConfig(ctx context.Context, roomID int64) (*DanmakuConfig, error) {
	return DefaultClient.GetDanmakuConfig(ctx, roomID)
}

func (c *Client) GetDanmakuConfig(ctx context.Context, roomID int64) (*DanmakuConfig, error) {
	if c.Credential == nil {
		return nil, ErrNeedLogin
	}

	if err := c.WbiKeys.update(ctx, c, false); err != nil {
		return nil, err
	}

	req, err := c.NewRequestWithContext(
		ctx,
		http.MethodGet,
		fmt.Sprintf("https://api.live.bilibili.com/xlive/web-room/v1/dM/GetDMConfigByGroup?room_id=%d", roomID),
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

	var n response[DanmakuConfig]
	if err := json.Unmarshal(body, &n); err != nil {
		return nil, err
	}

	data, err := n.DataOrError()
	if err != nil {
		return nil, err
	}

	return data, nil
}

type SentDanmakuInfo struct {
	Extra string `json:"extra"`
}

type DanmakuOption = func(o *danmakuOption)

type danmakuOption struct {
	msg      string
	replyMID int64
	mode     int64
	fontSize int64
	color    int64
}

func WithMsg(msg string) DanmakuOption {
	return func(o *danmakuOption) {
		o.msg = msg
	}
}

func WithReplyMID(replyMID int64) DanmakuOption {
	return func(o *danmakuOption) {
		o.replyMID = replyMID
	}
}

func WithMode(mode int64) DanmakuOption {
	return func(o *danmakuOption) {
		o.mode = mode
	}
}

func WithFontSize(fontSize int64) DanmakuOption {
	return func(o *danmakuOption) {
		o.fontSize = fontSize
	}
}

func WithColor(color int64) DanmakuOption {
	return func(o *danmakuOption) {
		o.color = color
	}
}

func getDefaultDanmakuOption() *danmakuOption {
	return &danmakuOption{
		mode:     1,        // 滚动弹幕
		fontSize: 25,       // 25px
		color:    16777215, // White
	}
}

func SendDanmaku(ctx context.Context, roomID int64, options ...DanmakuOption) error {
	return DefaultClient.SendDanmaku(ctx, roomID, options...)
}

func (c *Client) SendDanmaku(
	ctx context.Context,
	roomID int64,
	options ...DanmakuOption,
) error {
	option := getDefaultDanmakuOption()
	for _, f := range options {
		f(option)
	}

	if option.msg == "" {
		return errors.New("不能发送空弹幕")
	}

	if c.Credential == nil {
		return ErrNeedLogin
	}

	if err := c.WbiKeys.update(ctx, c, false); err != nil {
		return err
	}

	msgs := chunkMsg(option.msg, MAX_DANMAKU_MSG_LEN)

	var errs error
	for _, msg := range msgs {
		form := url.Values{
			"csrf":       {c.Credential.BiliJct},
			"csrf_token": {c.Credential.BiliJct},
			"roomid":     {strconv.FormatInt(roomID, 10)},
			"msg":        {msg},
			"rnd":        {strconv.FormatInt(time.Now().Unix(), 10)},
			"fontsize":   {strconv.FormatInt(option.fontSize, 10)},
			"color":      {strconv.FormatInt(option.color, 10)},
			"mode":       {strconv.FormatInt(option.mode, 10)},
		}

		if option.replyMID != 0 {
			form.Add("reply_mid", strconv.FormatInt(option.replyMID, 10))
		}

		req, err := c.NewRequestWithContext(
			ctx,
			http.MethodPost,
			"https://api.live.bilibili.com/msg/send",
			strings.NewReader(form.Encode()),
		)
		if err != nil {
			errs = errors.Join(errs, err)
			continue
		}

		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

		resp, err := c.Do(req)
		if err != nil {
			errs = errors.Join(errs, err)
			continue
		}
		defer resp.Body.Close()

		body, err := io.ReadAll(resp.Body)
		if err != nil {
			errs = errors.Join(errs, err)
			continue
		}

		var n response[SentDanmakuInfo]
		if err := json.Unmarshal(body, &n); err != nil {
			errs = errors.Join(errs, err)
			continue
		}

		_, err = n.DataOrError()
		if err != nil {
			errs = errors.Join(errs, err)
			continue
		}
	}

	return errs
}

func chunkMsg(msg string, numRunes int) []string {
	if numRunes <= 0 {
		return []string{msg}
	}

	var chunks []string
	var currentChunk []rune

	for _, r := range msg {
		currentChunk = append(currentChunk, r)

		if len(currentChunk) >= numRunes {
			chunks = append(chunks, string(currentChunk))
			currentChunk = nil
		}
	}

	// Add remaining characters
	if len(currentChunk) > 0 {
		chunks = append(chunks, string(currentChunk))
	}

	return chunks
}
