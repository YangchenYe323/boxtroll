package login

import (
	"encoding/json"
	"os"

	"github.com/spf13/cobra"
)

var Cmd = &cobra.Command{
	Use:   "login",
	Short: "登录Bilibili",
	Run: func(cmd *cobra.Command, args []string) {
		cred, err := DoLogin(cmd)
		if err != nil {
			cmd.PrintErrf("登录失败: %s\n", err.Error())
			os.Exit(1)
		}

		credsDir, err := cmd.Flags().GetString("creds-dir")
		if err != nil {
			panic("creds-dir flag is not defined")
		}

		if credsDir == "" {
			// Print to stdout and exit
			b, err := json.MarshalIndent(cred, "", "  ")
			if err != nil {
				// unreachable
				panic(err)
			}

			cmd.Println(string(b))
			os.Exit(0)
		}

		if err := SaveCredential(credsDir, cred); err != nil {
			cmd.PrintErrf("保存登录凭证失败: %s\n", err.Error())
			os.Exit(1)
		}

		cmd.Printf("登录凭证已保存到 %s\n", credsDir)
	},
}
