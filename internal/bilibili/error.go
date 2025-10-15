package bilibili

import (
	"errors"
	"fmt"
)

type APIError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

func (e APIError) Error() string {
	return fmt.Sprintf("code: %d, message: %s", e.Code, e.Message)
}

var (
	ErrNeedLogin = errors.New("请调用 Login 方法登录")
)
