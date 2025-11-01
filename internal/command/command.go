package command

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path"
	"runtime"

	"github.com/YangchenYe323/boxtroll/internal/bilibili"
	"github.com/YangchenYe323/boxtroll/internal/boxtroll"
	"github.com/YangchenYe323/boxtroll/internal/command/login"
	"github.com/YangchenYe323/boxtroll/internal/live"
	"github.com/YangchenYe323/boxtroll/internal/store"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"
	"gopkg.in/natefinch/lumberjack.v2"
)

// Global persistent flags
var (
	ROOT_DIR        string
	VEREBOSE        int
	LOG_MAX_SIZE    int
	LOG_MAX_BACKUPS int
	LOG_MAX_AGE     int
	ROOM_ID         int64
	SHOW_VERSION    bool
)

// Derived global flags
var (
	LOG_DIR   string
	DB_DIR    string
	CREDS_DIR string
)

const (
	// DB subdirectory
	DB_SUBDIR = "db"
	// Log subdirectory
	LOG_SUBDIR = "log"
	// Cached credentials subdirectory
	CREDS_SUBDIR = "creds"
)

var BoxtrollCmd = &cobra.Command{
	Use:   "boxtroll",
	Short: "统计直播间盲盒盈亏",
	PersistentPreRun: func(cmd *cobra.Command, args []string) {
		initializeGlobalFlags()
		cmd.Println("boxtroll - 统计直播间盲盒盈亏程序...")

		// Logging and working directories are initialized in PersistentPreRun hook
		// because subcommands also need these initialization work to finish.

		// Initialize logging
		initializeLogging()
		// Make sure all the working directories exist
		cmd.Printf("工作目录: %s ...\n", ROOT_DIR)
		if err := ensureDirs(); err != nil {
			log.Fatal().Err(err).Msg("无法创建工作目录")
		}
	},
	Run: RunBoxtroll,
}

func init() {
	BoxtrollCmd.PersistentFlags().StringVarP(&ROOT_DIR, "root-dir", "R", "", "改变boxtroll工作目录")
	BoxtrollCmd.PersistentFlags().CountVarP(&VEREBOSE, "verbose", "v", "输出日志的详细程度")
	BoxtrollCmd.PersistentFlags().IntVar(&LOG_MAX_SIZE, "log.max.size", 100, "日志文件的最大大小(MB)")
	BoxtrollCmd.PersistentFlags().IntVar(&LOG_MAX_BACKUPS, "log.max.backups", 10, "日志文件的最大备份数量")
	BoxtrollCmd.PersistentFlags().IntVar(&LOG_MAX_AGE, "log.max.age", 30, "日志文件的最大保存时间(天)")
	BoxtrollCmd.PersistentFlags().Int64VarP(&ROOM_ID, "roomid", "r", 0, "要监控的直播间ID")
	BoxtrollCmd.PersistentFlags().BoolVarP(&SHOW_VERSION, "version", "V", false, "显示版本信息")

	// These flags are needed so sub-commands located in different packages can access them
	// but we don't want the user to be able to set them, as they will be overridden anyway.
	// So we hide them from the help message.
	BoxtrollCmd.PersistentFlags().StringVarP(&LOG_DIR, "log-dir", "L", "", "Log directory")
	BoxtrollCmd.PersistentFlags().StringVarP(&DB_DIR, "db-dir", "D", "", "Database directory")
	BoxtrollCmd.PersistentFlags().StringVarP(&CREDS_DIR, "creds-dir", "C", "", "Credentials directory")
	BoxtrollCmd.PersistentFlags().MarkHidden("log-dir")
	BoxtrollCmd.PersistentFlags().MarkHidden("db-dir")
	BoxtrollCmd.PersistentFlags().MarkHidden("creds-dir")

	// Add sub-commands
	BoxtrollCmd.AddCommand(login.Cmd)
}

func RunBoxtroll(cmd *cobra.Command, args []string) {
	if SHOW_VERSION {
		cmd.Println("boxtroll version: ", Version)
		return
	}

	ctx := cmd.Context()

	// Initialize User
	uid, err := initializeUser(ctx, cmd)
	if err != nil {
		log.Fatal().Err(err).Msg("无法初始化用户")
	}
	log.Info().Int64("uid", uid).Msg("用户初始化成功")

	// Ininitialize Room ID
	if ROOM_ID == 0 {
		// Prompt user to input room ID
		cmd.Print("请输入直播间号: ")
		_, err := fmt.Scanln(&ROOM_ID)
		if err != nil {
			log.Fatal().Err(err).Msg("无法获取直播间号")
		}
	}

	// Fetch message stream info for the given live room
	streamInfo, err := bilibili.GetMessageStreamInfo(ctx, ROOM_ID)
	if err != nil {
		log.Fatal().Err(err).Msg("无法获取直播间弹幕流信息")
	}
	stream := live.NewStream(ROOM_ID, uid, streamInfo.Token, streamInfo.HostList)

	s, err := store.NewBadger(DB_DIR)
	if err != nil {
		log.Fatal().Err(err).Msg("无法初始化数据库")
	}

	boxtroll := boxtroll.New(s, stream)
	boxtroll.Run(ctx)
}

// Initialize verified user credential and return the UID of the credential holder.
// It is the Bilibili user that will connect to the live stream and send danmaku.
func initializeUser(ctx context.Context, cmd *cobra.Command) (int64, error) {
	// First try cached credential
	cred, err := login.GetCachedCredential(CREDS_DIR)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return -1, err
	}

	if err == nil {
		uid, err := initializeBilibili(ctx, cred)
		if err == nil {
			return uid, nil
		}

		var apiErr bilibili.APIError
		if errors.As(err, &apiErr) {
			if apiErr.Code == -101 {
				// Credential has expired, proceed to login
				cmd.Println("登录凭证已过期，重新登录...")
			}
		}

		return -1, err
	}

	credential, err := login.DoLogin(cmd)
	if err != nil {
		return -1, err
	}

	// Save credential to file
	if err := login.SaveCredential(CREDS_DIR, credential); err != nil {
		log.Warn().Err(err).Msg("无法保存登录凭证，您下次登录时需要重新扫描二维码")
	}

	return initializeBilibili(ctx, credential)
}

func initializeBilibili(ctx context.Context, credential *bilibili.Credential) (int64, error) {
	buvid, err := bilibili.GetBuvid(ctx)
	if err != nil {
		return -1, err
	}

	credential.Buvid3 = buvid.B3
	bilibili.Login(credential)

	user, err := bilibili.GetUserInfo(ctx)
	if err != nil {
		return -1, err
	}

	return user.MID, nil
}

func initializeLogging() {
	var level zerolog.Level
	switch VEREBOSE {
	case 0:
		level = zerolog.InfoLevel
	case 1:
		level = zerolog.DebugLevel
	default:
		level = zerolog.TraceLevel
	}

	zerolog.SetGlobalLevel(level)

	logRotater := lumberjack.Logger{
		Filename:   path.Join(LOG_DIR, "boxtroll.log"),
		MaxSize:    LOG_MAX_SIZE,
		MaxBackups: LOG_MAX_BACKUPS,
		MaxAge:     LOG_MAX_AGE,
	}

	// Write logs to both stderr and lumberjack log rotater
	multiWriter := zerolog.MultiLevelWriter(
		zerolog.ConsoleWriter{Out: os.Stderr},
		&logRotater,
	)

	log.Logger = log.Output(multiWriter)
}

func ensureDirs() error {
	if err := os.MkdirAll(ROOT_DIR, 0755); err != nil {
		return err
	}
	if err := os.MkdirAll(DB_DIR, 0755); err != nil {
		return err
	}
	if err := os.MkdirAll(LOG_DIR, 0755); err != nil {
		return err
	}
	if err := os.MkdirAll(CREDS_DIR, 0755); err != nil {
		return err
	}

	return nil
}

func initializeGlobalFlags() {
	if ROOT_DIR == "" {
		rootDir, err := getDefaultRootDir()
		if err != nil {
			panic(fmt.Sprintf("无法获取默认根目录: %s, 请指定 --root 参数或环境变量 BOXMON_ROOT", err.Error()))
		}
		ROOT_DIR = path.Join(rootDir, ".boxtroll")
	}

	LOG_DIR = path.Join(ROOT_DIR, LOG_SUBDIR)
	DB_DIR = path.Join(ROOT_DIR, DB_SUBDIR)
	CREDS_DIR = path.Join(ROOT_DIR, CREDS_SUBDIR)
}

func getDefaultRootDir() (string, error) {
	if rootDir := os.Getenv("BOXMON_ROOT"); rootDir != "" {
		return rootDir, nil
	}

	// Try to find a suitable base directory based on OS we run on.
	if runtime.GOOS == "windows" {
		// On windoes we try:
		// - $env:LOCALAPPDATA
		// - $HOME
		if localAppData := os.Getenv("LOCALAPPDATA"); localAppData != "" {
			return localAppData, nil
		}

		return os.UserHomeDir()
	}

	// On unix we try:
	// - $XDG_DATA_HOME
	// - $HOME
	if xdgDataHome := os.Getenv("XDG_DATA_HOME"); xdgDataHome != "" {
		return xdgDataHome, nil
	}

	return os.UserHomeDir()
}
