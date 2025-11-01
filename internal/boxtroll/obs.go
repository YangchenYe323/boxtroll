package boxtroll

import (
	"context"
	"fmt"
	"slices"
	"strings"

	"github.com/andreykaipov/goobs"
	"github.com/andreykaipov/goobs/api/requests/inputs"
	"github.com/rs/zerolog/log"
)

const (
	OBS_SOURCE_NAME = "boxtroll"
)

func (b *Boxtroll) initializeOBS(ctx context.Context) error {
	versionRes, err := b.obs.General.GetVersion()
	if err != nil {
		return fmt.Errorf("无法获取OBS版本信息: %w", err)
	}

	log.Info().
		Str("obs.websocket.version", versionRes.ObsWebSocketVersion).
		Str("obs.version", versionRes.ObsVersion).
		Str("platform", versionRes.Platform).
		Float64("rpc.version", versionRes.RpcVersion).
		Msg("OBS版本信息")

	// Fetch available input source kind. For now just use text
	kindsResp, err := b.obs.Inputs.GetInputKindList()
	if err != nil {
		return fmt.Errorf("无法获取输入源种类: %w", err)
	}
	for _, kind := range kindsResp.InputKinds {
		if strings.Contains(kind, "text") {
			b.inputSourceKind = kind
			break
		}
	}

	if b.inputSourceKind == "" {
		return fmt.Errorf("无法找到文本输入源种类")
	}

	log.Info().Str("input.source.kind", b.inputSourceKind).Msg("找到文本输入源种类")

	// Get the current program scene
	sceneResp, err := b.obs.Scenes.GetCurrentProgramScene()
	if err != nil {
		return fmt.Errorf("无法获取当前节目场景: %w", err)
	}
	b.sceneName = sceneResp.CurrentProgramSceneName
	log.Info().Str("scene.name", b.sceneName).Msg("使用节目场景")

	// Create a text source for showing:
	// - 本场直播盲盒盈亏排行榜
	// - to be added
	b.sourceName = OBS_SOURCE_NAME

	sceneItemEnabled := true
	createReq := &inputs.CreateInputParams{
		SceneName: &b.sceneName,
		InputName: &b.sourceName,
		InputKind: &b.inputSourceKind,
		InputSettings: map[string]interface{}{
			"text": "",
			"font": map[string]interface{}{
				"face":  "Arial",
				"size":  36,
				"flags": 1, // Bold
			},
			"color":         0xFFFFFFFF, // White color (ARGB)
			"outline":       true,
			"outline_size":  2,
			"outline_color": 0xFF000000, // Black outline
		},
		SceneItemEnabled: &sceneItemEnabled,
	}

	createResp, err := b.obs.Inputs.CreateInput(createReq)
	if err != nil {
		// 601 - Resource already exists
		if strings.Contains(err.Error(), "601") {
			log.Info().Msg("文本输入源已存在，跳过创建")
		} else {
			return fmt.Errorf("无法创建文本输入源: %w", err)
		}
	}
	log.Info().Int64("scene.item.id", int64(createResp.SceneItemId)).Msg("创建文本输入源成功")

	return nil
}

func (b *Boxtroll) updateOBS(ctx context.Context) {
	var err error

	if b.reconnect {
		b.obs, err = goobs.New(b.obsAddr, goobs.WithPassword(b.obsPassword))
		if err != nil {
			log.Err(err).Msg("无法连接到OBS websocket")
			return
		}
		log.Info().Msg("OBS websocket 重新连接成功")
		b.reconnect = false
	}

	report := b.generateOBSReport(ctx)

	if report == "" {
		return
	}

	updateReq := &inputs.SetInputSettingsParams{
		InputName: &b.sourceName,
		InputSettings: map[string]interface{}{
			"text": report,
		},
	}

	if _, err := b.obs.Inputs.SetInputSettings(updateReq); err != nil {
		if strings.Contains(err.Error(), "disconnected") {
			b.reconnect = true
			log.Warn().Msg("OBS websocket 连接断开，重新连接中...")
			return
		}

		log.Err(err).Msg("无法更新OBS文本输入源")
	}
}

func (b *Boxtroll) generateOBSReport(ctx context.Context) string {
	type userAggregateReport struct {
		uid         int64
		name        string
		diffBattery int64
	}

	type report struct {
		topFiveLuckyUsers   []*userAggregateReport
		topFiveUnluckyUsers []*userAggregateReport
	}

	var reports report

	for uid, boxIDMap := range b.curStreamSt {
		user, err := b.db.GetUser(ctx, uid)
		if err != nil {
			// Should not happen but don't panic
			log.Warn().Err(err).Int64("uid", uid).Msg("未知的盲盒用户")
			continue
		}

		diff := int64(0)
		for _, st := range boxIDMap {
			diff += st.TotalPrice - st.TotalOriginalPrice
		}

		diffBattery := diff / 100

		if diffBattery > 0 {
			reports.topFiveLuckyUsers = append(reports.topFiveLuckyUsers, &userAggregateReport{
				uid:         uid,
				name:        user.Name,
				diffBattery: diffBattery,
			})
		} else if diffBattery < 0 {
			reports.topFiveUnluckyUsers = append(reports.topFiveUnluckyUsers, &userAggregateReport{
				uid:         uid,
				name:        user.Name,
				diffBattery: diffBattery,
			})
		}
	}

	// Sort in descending
	slices.SortFunc(reports.topFiveLuckyUsers, func(a *userAggregateReport, b *userAggregateReport) int {
		return int(a.diffBattery - b.diffBattery)
	})
	// Sort in ascending
	slices.SortFunc(reports.topFiveUnluckyUsers, func(a *userAggregateReport, b *userAggregateReport) int {
		return int(b.diffBattery - a.diffBattery)
	})

	if len(reports.topFiveLuckyUsers) > 5 {
		reports.topFiveLuckyUsers = reports.topFiveLuckyUsers[:5]
	}
	if len(reports.topFiveUnluckyUsers) > 5 {
		reports.topFiveUnluckyUsers = reports.topFiveUnluckyUsers[:5]
	}

	var sb strings.Builder

	sb.WriteString("本场盲盒幸运儿排行榜: \n")
	for i := range 5 {
		sb.WriteString(fmt.Sprintf("%d. ", i+1))
		if i < len(reports.topFiveLuckyUsers) {
			sb.WriteString(fmt.Sprintf("%s: +%d 电池\n", reports.topFiveLuckyUsers[i].name, reports.topFiveLuckyUsers[i].diffBattery))
		} else {
			sb.WriteString("暂无~\n")
		}
	}

	sb.WriteString("本场盲盒倒霉蛋排行榜: \n")
	for i := range 5 {
		sb.WriteString(fmt.Sprintf("%d. ", i+1))
		if i < len(reports.topFiveUnluckyUsers) {
			sb.WriteString(fmt.Sprintf("%s: %d 电池\n", reports.topFiveUnluckyUsers[i].name, reports.topFiveUnluckyUsers[i].diffBattery))
		} else {
			sb.WriteString("暂无~\n")
		}
	}

	return sb.String()
}
