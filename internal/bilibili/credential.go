package bilibili

type Credential struct {
	SessionData     string `json:"sess_data"`
	BiliJct         string `json:"bili_jct"`
	DedeUserID      string `json:"dede_userid"`
	DedeUserIDCkMd5 string `json:"dede_userid_ck_md5"`
	Buvid3          string `json:"buvid3"`
}
