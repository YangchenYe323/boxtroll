package boxtroll

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/YangchenYe323/boxtroll/internal/bilibili"
	"github.com/YangchenYe323/boxtroll/internal/live"
	"github.com/YangchenYe323/boxtroll/internal/store"
	"github.com/YangchenYe323/boxtroll/internal/throttle"
	"github.com/andreykaipov/goobs"
	"github.com/rs/zerolog/log"
)

// Boxtroll is the driver of the application.
type Boxtroll struct {
	db *boxtrollStore
	// Live Stream for receiving danmaku/gift messages
	stream *live.Stream
	// Throttler for sending danmaku to Bilibili to avoid rate limiting
	throttler *throttle.Throttler

	// OBS Websocket connection for updating text inputs
	obsAddr         string
	obsPassword     string
	obs             *goobs.Client
	inputSourceKind string
	sceneName       string
	sourceName      string
	// Signal that the remote OBS websocket connection is lost. We will reconnect in the next update
	reconnect bool
	reportIdx int64

	// State of the current live stream
	cuStreamStMutex sync.RWMutex

	// Local state of the main event loop.
	// The below fields are owned by the main event loop and should not be accessed by background goroutines.

	// Stores the statistics of the current live stream
	curStreamSt map[int64]map[int64]*store.BoxStatistics
	// 存储当前直播间用户的电影票数量
	curTicketNum map[int64]int64
	// Temporary store, stores statistics of the current accumulating batch
	// uid -> boxID -> statistics
	// This is NOT the same as the store.BoxStatisticsCache in the store, which stores the accumulation of
	// all the box statistics.
	curBatch map[int64]map[int64]*store.BoxStatistics
	// A most up-to-date map of boxIDs to box names kept in sync with the ongoing live stream messages.
	// Box Gift ID -> Box Gift Name, e.g., 心动盲盒.
	// Even though we do store box information inside theb data store updated on every start-up, it is not guaranteed
	// to be up-to-date. For example, if a new box is released while the live stream is going on, the new box will not
	// have been updated in the data store. We keep a cutting-edge-fresh cache here to power live danmaku reporting
	// and use the persisted metadata for asynchronous reports.
	boxNames map[int64]string
}

func New(ctx context.Context, db store.Store, stream *live.Stream, obsAddr string, obsPassword string, obs *goobs.Client) (*Boxtroll, error) {
	log.Info().Msg("启动盒子怪，更新直播间和用户信息...")

	_, err := refreshRoom(ctx, db, stream.RoomID)
	if err != nil {
		return nil, fmt.Errorf("无法刷新直播间信息: %w", err)
	}

	if err := refreshAllUsers(ctx, db, stream.RoomID); err != nil {
		return nil, fmt.Errorf("无法刷新所有用户信息: %w", err)
	}

	log.Info().Msg("直播间和用户信息更新完成")

	boxtrollStore, err := newBoxtrollStore(ctx, db, stream.RoomID)
	if err != nil {
		return nil, err
	}

	return &Boxtroll{
		db:          boxtrollStore,
		stream:      stream,
		obsAddr:     obsAddr,
		obsPassword: obsPassword,
		obs:         obs,
		// Bilibili has a pretty stringent and not so predictable rate limit for
		// sending danmaku, we do ((0.8, 1.2) * 2) * seconds throttle
		throttler: throttle.New(1600*time.Millisecond, 2400*time.Millisecond),

		curBatch:     make(map[int64]map[int64]*store.BoxStatistics),
		curStreamSt:  make(map[int64]map[int64]*store.BoxStatistics),
		curTicketNum: make(map[int64]int64),
		boxNames:     make(map[int64]string),
	}, nil
}

func (b *Boxtroll) Run(ctx context.Context) {
	msgChan := make(chan live.Message, 100)

	go b.stream.Run(ctx, msgChan)

	var obsTimer *time.Ticker
	if b.obs != nil {
		obsTimer = time.NewTicker(5 * time.Second)
		if err := b.initializeOBS(ctx); err != nil {
			log.Fatal().Err(err).Msg("无法初始化OBS")
		}
	} else {
		obsTimer = time.NewTicker(time.Hour * 9999)
		obsTimer.Stop()
	}

	for {
		if err := b.flushBatch(ctx); err != nil {
			log.Fatal().Err(err).Msg("无法处理已完成的盲盒数据批次")
		}

		select {
		case <-ctx.Done():
			return
		case msg := <-msgChan:
			b.handleMessage(msg)
		case <-obsTimer.C:
			// If obs is nil, timer will never fire
			b.updateOBS(ctx)
		case <-time.After(2 * time.Second):
		}
	}
}

// An entry describing a finished box batch to be flushed
type finishedBatch struct {
	key     []byte
	uid     int64
	boxID   int64
	boxName string
	st      store.BoxStatistics
	accumSt store.BoxStatistics
}

// Implement store.BoxStatisticsTransfer interface
func (f *finishedBatch) Key() []byte {
	return f.key
}

// Implement store.BoxStatisticsTransfer interface
func (f *finishedBatch) GetBoxStatistics() *store.BoxStatistics {
	return &f.accumSt
}

// Flush finished batches to database and send danmaku report to Bilibili.
//
// It only fails if we cannot finish the database transactions, in which case something
// is probably wrong with local disk. It does NOT fail if we cannot send danmaku, which is
// more or less out of our control and might recover by itself.
func (b *Boxtroll) flushBatch(ctx context.Context) error {
	var entries []*finishedBatch
	for uid, boxIDMap := range b.curBatch {
		go func(uid int64) {
			if err := b.createUserIfNotExists(ctx, uid); err != nil {
				log.Err(err).Int64("uid", uid).Msg("无法创建用户")
			}
		}(uid)

		for boxID, st := range boxIDMap {
			// Since we populate the box names upon seeing a SEND_GIFT msg, and populate
			// current batch in the same place, it is impossible for boxName to be nil
			boxName := b.boxNames[boxID]

			if st.LastUpdateTime.IsZero() {
				continue
			}
			if time.Since(st.LastUpdateTime) < time.Second {
				continue
			}

			entries = append(entries, &finishedBatch{
				key:     b.db.BoxStatisticsKey(b.stream.RoomID, uid, boxID),
				uid:     uid,
				boxID:   boxID,
				boxName: boxName,
				st:      *st,
			})

			st.Reset()
		}
	}

	var transfers []store.BoxStatisticsTransfer
	for _, entry := range entries {
		transfers = append(transfers, entry)
	}

	if err := b.db.GetBoxStatistics(ctx, transfers, store.NotFoundBehaviorSkip); err != nil {
		return err
	}

	// Merging the current batch
	for _, entry := range entries {
		entry.accumSt.Merge(entry.st)
	}

	if err := b.db.SetBoxStatistics(ctx, transfers); err != nil {
		return err
	}

	go b.sendDanmakuReport(ctx, entries)

	return nil
}

func (b *Boxtroll) createUserIfNotExists(ctx context.Context, uid int64) error {
	_, err := b.db.GetUser(ctx, uid)

	if errors.Is(err, store.ErrNotFound) {
		user, err := bilibili.GetUserInfo(ctx, uid)
		if err != nil {
			return err
		}

		if err := b.db.SetUser(ctx, uid, &store.User{
			MID:  uid,
			Name: user.Name,
			Face: user.Face,
		}); err != nil {
			return err
		}
		return nil
	}

	return nil
}

func (b *Boxtroll) sendDanmakuReport(
	ctx context.Context,
	entries []*finishedBatch,
) {
	for _, entry := range entries {
		// 当前batch盈亏
		curDiff := entry.st.TotalPrice - entry.st.TotalOriginalPrice
		// 历史总盈亏
		accumDiff := entry.accumSt.TotalPrice - entry.accumSt.TotalOriginalPrice

		curDiffBattery := curDiff / 100
		accumDiffBattery := accumDiff / 100

		msgs := []string{
			danmaku(entry.boxName, curDiffBattery, false),
			danmaku(entry.boxName, accumDiffBattery, true),
		}

		for _, msg := range msgs {
			if err := b.throttler.Run(func() error {
				return bilibili.SendDanmaku(
					ctx,
					b.stream.RoomID,
					bilibili.WithMsg(msg),
					bilibili.WithReplyMID(entry.uid),
				)
			}); err != nil {
				log.Err(err).Str("danmaku", msg).Msg("发送弹幕失败")
			}
		}

	}
}

func (b *Boxtroll) handleMessage(msg live.Message) {
	switch msg.Cmd {
	case "SEND_GIFT":
		b.handleSendGift(msg.SendGift)
	}
}

func (b *Boxtroll) handleSendGift(sendGift *live.SendGiftMessage) {
	if sendGift.BlindGift == nil {
		return
	}

	if _, ok := b.curBatch[sendGift.UID]; !ok {
		b.curBatch[sendGift.UID] = make(map[int64]*store.BoxStatistics)
	}
	if _, ok := b.curBatch[sendGift.UID][sendGift.BlindGift.OriginalGiftID]; !ok {
		b.curBatch[sendGift.UID][sendGift.BlindGift.OriginalGiftID] = &store.BoxStatistics{}
	}
	if _, ok := b.curStreamSt[sendGift.UID]; !ok {
		b.curStreamSt[sendGift.UID] = make(map[int64]*store.BoxStatistics)
	}
	if _, ok := b.curStreamSt[sendGift.UID][sendGift.BlindGift.OriginalGiftID]; !ok {
		b.curStreamSt[sendGift.UID][sendGift.BlindGift.OriginalGiftID] = &store.BoxStatistics{}
	}

	// Update current unsent batch
	st := b.curBatch[sendGift.UID][sendGift.BlindGift.OriginalGiftID]
	st.TotalNum += sendGift.Num
	st.TotalOriginalPrice += sendGift.BlindGift.OriginalGiftPrice * sendGift.Num
	st.TotalPrice += sendGift.Price * sendGift.Num
	st.LastUpdateTime = time.Now()

	// Update current stream statistics
	curSt := b.curStreamSt[sendGift.UID][sendGift.BlindGift.OriginalGiftID]
	curSt.TotalNum += sendGift.Num
	curSt.TotalOriginalPrice += sendGift.BlindGift.OriginalGiftPrice * sendGift.Num
	curSt.TotalPrice += sendGift.Price * sendGift.Num
	curSt.LastUpdateTime = time.Now()

	// Populate the box names lazily
	if _, ok := b.boxNames[sendGift.BlindGift.OriginalGiftID]; !ok {
		b.boxNames[sendGift.BlindGift.OriginalGiftID] = sendGift.BlindGift.OriginalGiftName
	}

	// Update current ticket number
	if sendGift.GiftName == "电影票" {
		b.curTicketNum[sendGift.UID] += sendGift.Num
	}
}

func danmaku(boxName string, diffBattery int64, accum bool) string {
	var prefix string
	if accum {
		prefix = "历史"
	}

	if diffBattery >= 0 {
		return fmt.Sprintf("%s投喂 %s: +%d 电池", prefix, boxName, diffBattery)
	} else {
		return fmt.Sprintf("%s投喂 %s: %d 电池", prefix, boxName, diffBattery)
	}
}
