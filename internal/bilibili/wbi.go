package bilibili

import (
	"bytes"
	"context"
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

type WbiKeys struct {
	ImgKey     string    `json:"img_key"`
	SubKey     string    `json:"sub_key"`
	Mixin      string    `json:"mixin"`
	LastUpdate time.Time `json:"last_update"`
}

var DefaultWbiKeys = &WbiKeys{}

// Sign 为链接签名
func (wk *WbiKeys) Sign(u *url.URL) (err error) {
	values := u.Query()
	values = removeUnwantedChars(values, '!', '\'', '(', ')', '*') // 必要性存疑?

	values.Set("wts", strconv.FormatInt(time.Now().Unix(), 10))

	// [url.Values.Encode] 内会对参数排序,
	// 且遍历 map 时本身就是无序的
	hash := md5.Sum([]byte(values.Encode() + wk.Mixin)) // Calculate w_rid
	values.Set("w_rid", hex.EncodeToString(hash[:]))
	u.RawQuery = values.Encode()
	return nil
}

func (wk *WbiKeys) Update(ctx context.Context) error {
	return wk.update(ctx, DefaultClient, false)
}

func (wk *WbiKeys) update(ctx context.Context, client *Client, purge bool) error {
	if wk == nil {
		wk = &WbiKeys{}
	}

	type wbiData struct {
		WbiImg struct {
			ImgURL string `json:"img_url"`
			SubURL string `json:"sub_url"`
		} `json:"wbi_img"`
	}

	if !purge && time.Since(wk.LastUpdate) < time.Hour {
		return nil
	}

	req, err := client.NewRequestWithContext(ctx, http.MethodGet, "https://api.bilibili.com/x/web-interface/nav", nil)
	if err != nil {
		return err
	}

	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}

	var n response[wbiData]
	if err := json.Unmarshal(body, &n); err != nil {
		return fmt.Errorf("failed to unmarshal wbi data %s: %w", string(body), err)
	}

	data, err := n.DataOrError(0, -101)
	if err != nil {
		return err
	}

	img := data.WbiImg.ImgURL
	sub := data.WbiImg.SubURL
	if img == "" || sub == "" {
		return fmt.Errorf("empty image or sub url: %s", body)
	}

	// https://i0.hdslb.com/bfs/wbi/7cd084941338484aae1ad9425b84077c.png
	imgParts := strings.Split(img, "/")
	subParts := strings.Split(sub, "/")

	// 7cd084941338484aae1ad9425b84077c.png
	imgPng := imgParts[len(imgParts)-1]
	subPng := subParts[len(subParts)-1]

	// 7cd084941338484aae1ad9425b84077c
	wk.ImgKey = strings.TrimSuffix(imgPng, ".png")
	wk.SubKey = strings.TrimSuffix(subPng, ".png")

	wk.mixin()
	wk.LastUpdate = time.Now()
	return nil
}

func (wk *WbiKeys) mixin() {
	var mixin [32]byte
	wbi := wk.ImgKey + wk.SubKey
	for i := range mixin { // for i := 0; i < len(mixin); i++ {
		mixin[i] = wbi[mixinKeyEncTab[i]]
	}
	wk.Mixin = string(mixin[:])
}

var mixinKeyEncTab = [...]int{
	46, 47, 18, 2, 53, 8, 23, 32,
	15, 50, 10, 31, 58, 3, 45, 35,
	27, 43, 5, 49, 33, 9, 42, 19,
	29, 28, 14, 39, 12, 38, 41, 13,
	37, 48, 7, 16, 24, 55, 40, 61,
	26, 17, 0, 1, 60, 51, 30, 4,
	22, 25, 54, 21, 56, 59, 6, 63,
	57, 62, 11, 36, 20, 34, 44, 52,
}

func removeUnwantedChars(v url.Values, chars ...byte) url.Values {
	b := []byte(v.Encode())
	for _, c := range chars {
		b = bytes.ReplaceAll(b, []byte{c}, nil)
	}
	s, err := url.ParseQuery(string(b))
	if err != nil {
		panic(err)
	}
	return s
}
