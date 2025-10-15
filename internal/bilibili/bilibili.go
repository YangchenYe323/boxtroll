package bilibili

import (
	"context"
	"net/http"
)

func Login(credential *Credential) {
	DefaultClient.Login(credential)
}

func Get(ctx context.Context, url string) (*http.Response, error) {
	return DefaultClient.Get(url)
}

func Do(ctx context.Context, req *http.Request) (*http.Response, error) {
	return DefaultClient.Do(req)
}
