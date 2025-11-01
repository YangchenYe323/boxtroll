package boxtroll

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/YangchenYe323/boxtroll/internal/bilibili"
	"github.com/YangchenYe323/boxtroll/internal/store"
	"github.com/rs/zerolog/log"
)

func refreshAllUsers(ctx context.Context, s store.Store, roomID int64) error {
	var userIDSet = make(map[int64]struct{})

	// Refresh all known users
	userIDs, err := s.ListAllUserIDs(ctx)
	if err != nil {
		return fmt.Errorf("无法获取所有用户ID: %w", err)
	}

	// Refresh all users that have sent box gifts in the given room
	boxSenderUserIDs, err := s.ListAllBoxSenderUserIDs(ctx, roomID)
	if err != nil {
		return fmt.Errorf("无法获取所有发送盲盒礼物的用户ID: %w", err)
	}

	for _, userID := range userIDs {
		userIDSet[userID] = struct{}{}
	}
	for _, userID := range boxSenderUserIDs {
		userIDSet[userID] = struct{}{}
	}

	for userID := range userIDSet {
		log.Info().Int64("uid", userID).Msg("获取用户信息...")

		user, err := bilibili.GetUserInfo(ctx, userID)
		if err != nil {
			return fmt.Errorf("无法获取用户信息: %w", err)
		}

		if err := s.SetUser(ctx, userID, &store.User{
			MID:  userID,
			Name: user.Name,
			Face: user.Face,
		}); err != nil {
			return fmt.Errorf("无法保存用户信息: %w", err)
		}

		log.Info().Int64("uid", userID).Str("name", user.Name).Msg("成功获取最新用户信息")
	}

	return nil
}

func refreshRoom(ctx context.Context, s store.Store, roomID int64) (*store.Room, error) {
	log.Info().Str("room_id", strconv.FormatInt(roomID, 10)).Msg("获取最新直播间信息...")
	log.Info().Str("room_id", strconv.FormatInt(roomID, 10)).Msg("获取最新直播间礼物配置...")

	var room = store.Room{
		RoomID: roomID,
		Gifts:  make([]*store.Gift, 0),
	}

	giftConfig, err := bilibili.GetLiveRoomGift(ctx, roomID)
	if err != nil {
		return nil, fmt.Errorf("无法获取直播间礼物配置: %w", err)
	}

	for _, gift := range giftConfig.GiftConfig.BaseConfig.GiftList {
		g := &store.Gift{
			GiftID:   gift.ID,
			Name:     gift.Name,
			Price:    gift.Price,
			CoinType: gift.CoinType,
			ImgURL:   gift.ImgURL,
		}

		if strings.Contains(gift.Name, "盲盒") {
			log.Info().Str("name", gift.Name).Int64("id", gift.ID).Msg("获取盲盒配置")
			blindBoxConfig, err := bilibili.GetBlindBoxConfig(ctx, gift.ID)
			if err != nil {
				return nil, fmt.Errorf("无法获取盲盒配置: %w", err)
			}
			g.BlindBoxOutcomes = make([]store.BlindBoxOutcome, 0)
			for _, outcome := range blindBoxConfig.OutcomeGifts {
				g.BlindBoxOutcomes = append(g.BlindBoxOutcomes, store.BlindBoxOutcome{
					GiftID: outcome.ID,
					Price:  outcome.Price,
					Name:   outcome.Name,
					ImgURL: outcome.ImgURL,
					Chance: outcome.Chance,
				})
			}
		}

		room.Gifts = append(room.Gifts, g)
	}

	if err := s.SetRoom(ctx, roomID, &room); err != nil {
		return nil, fmt.Errorf("无法保存直播间信息: %w", err)
	}

	return &room, nil
}
