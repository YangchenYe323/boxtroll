package login

import (
	"time"

	"github.com/YangchenYe323/boxtroll/internal/bilibili"
	"github.com/mdp/qrterminal/v3"
	"github.com/spf13/cobra"
)

// Drive interactive login.
//  1. Request a login QR code and display it.
//  2. Poll login status:
//     2.1: If user has not scanned, prompt user to scan.
//     2.2: If user has scanned, prompt for login confirmation.
//     2.3: If user has logged in, return the credential.
//     2.4: If code expired, go back to 1
func DoLogin(cmd *cobra.Command) (*bilibili.Credential, error) {
	ctx := cmd.Context()

	for {
		cmd.Println("获取 Bilibili 登录二维码...")
		qrCode, err := bilibili.GetLoginQRCode(ctx)
		if err != nil {
			return nil, err
		}

		qrCodeURL := qrCode.URL
		qrCodeKey := qrCode.Key
		qrterminal.Generate(qrCodeURL, qrterminal.M, cmd.OutOrStdout())
		cmd.Print("请使用手机 Bilibili 扫码登录")

	poll:
		for {
			result, cred, err := bilibili.PollLogin(ctx, qrCodeKey)
			if err != nil {
				return nil, err
			}

			switch result.Code {
			case bilibili.LoginStatusSuccess:
				cmd.Print("\r登录成功")
				cmd.Println()
				return cred, nil
			case bilibili.LoginStatusCodeExpired:
				cmd.Print("\r登录二维码失效, 重新登录...")
				break poll
			case bilibili.LoginStatusCodeScanned:
				cmd.Print("\r请在手机 Bilibili 上确认登录")
			case bilibili.LoginStatusCodeUnscanned:
				cmd.Print("\r请使用手机 Bilibili 扫码登录")
			}

			time.Sleep(1 * time.Second)
		}
	}
}
