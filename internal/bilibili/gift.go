package bilibili

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

type LiveRoomGift struct {
	GiftConfig *GiftConfig `json:"gift_config"`
}

type GiftConfig struct {
	BaseConfig *BaseConfig `json:"base_config"`
}

type BaseConfig struct {
	GiftList []*Gift `json:"list"`
}

type Gift struct {
	ID       int64  `json:"id"`
	Name     string `json:"name"`
	Price    int64  `json:"price"`
	CoinType string `json:"coin_type"`
	ImgURL   string `json:"img_basic"`
}

func GetLiveRoomGift(ctx context.Context, roomID int64) (*LiveRoomGift, error) {
	return DefaultClient.GetLiveRoomGift(ctx, roomID)
}

func (c *Client) GetLiveRoomGift(ctx context.Context, roomID int64) (*LiveRoomGift, error) {
	req, err := c.NewRequestWithContext(
		ctx,
		http.MethodGet,
		fmt.Sprintf("https://api.live.bilibili.com/xlive/web-room/v1/giftPanel/roomGiftList?platform=pc&room_id=%d", roomID),
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

	var n response[LiveRoomGift]
	if err := json.Unmarshal(body, &n); err != nil {
		return nil, err
	}

	data, err := n.DataOrError()
	if err != nil {
		return nil, err
	}

	return data, nil
}

type BlindBoxConfig struct {
	NodeText      string                 `json:"node_text"`
	BlindPrice    int64                  `json:"blind_price"`
	BlindGiftName string                 `json:"blind_gift_name"`
	OutcomeGifts  []*BlindBoxOutcomeGift `json:"gifts"`
}

type BlindBoxOutcomeGift struct {
	ID     int64  `json:"gift_id"`
	Name   string `json:"gift_name"`
	Price  int64  `json:"price"`
	ImgURL string `json:"gift_img"`
	Chance string `json:"chance"`
}

func GetBlindBoxConfig(ctx context.Context, giftID int64) (*BlindBoxConfig, error) {
	return DefaultClient.GetBlindBoxConfig(ctx, giftID)
}

func (c *Client) GetBlindBoxConfig(ctx context.Context, giftID int64) (*BlindBoxConfig, error) {
	req, err := c.NewRequestWithContext(
		ctx,
		http.MethodGet,
		fmt.Sprintf("https://api.live.bilibili.com/xlive/general-interface/v1/blindFirstWin/getInfo?gift_id=%d", giftID),
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

	var n response[BlindBoxConfig]
	if err := json.Unmarshal(body, &n); err != nil {
		return nil, err
	}

	data, err := n.DataOrError()
	if err != nil {
		return nil, err
	}

	return data, nil
}
