package boxtroll

import (
	"context"
	"fmt"
	"time"

	"github.com/YangchenYe323/boxtroll/internal/bilibili"
	"github.com/YangchenYe323/boxtroll/internal/live"
	"github.com/YangchenYe323/boxtroll/internal/store"
	"github.com/YangchenYe323/boxtroll/internal/throttle"
	"github.com/rs/zerolog/log"
)

type Boxtroll struct {
	db     store.Store
	stream *live.Stream

	// Temporary store, stores statistics of the current accumulating batch
	// uid -> boxID -> statistics
	curBatch map[int64]map[int64]*store.BoxStatistics
	// Box Gift ID -> Box Gift Name, e.g., 心动盲盒.
	// Currently we do not persist box names, because the only way we send danmaku now
	// is when user actually send a gift, in which case we always get the fresh name of
	// the blind box.
	// In the future if we want to extend the functionality, e.g., by implementing control
	// messages to get historical data, etc., we'd need a separate channel for persisting and
	// caching metadata like gift name, username, etc.
	boxNames map[int64]string

	// Throttler for sending danmaku
	throttler *throttle.Throttler
}

func New(db store.Store, stream *live.Stream) *Boxtroll {
	return &Boxtroll{
		db:       db,
		stream:   stream,
		curBatch: make(map[int64]map[int64]*store.BoxStatistics),
		boxNames: make(map[int64]string),
		// Bilibili has a pretty stringent and not so predictable rate limit for
		// sending danmaku, we do ((0.8, 1.2) * 2) * seconds throttle
		throttler: throttle.New(1600*time.Millisecond, 2400*time.Millisecond),
	}
}

func (b *Boxtroll) Run(ctx context.Context) {
	msgChan := make(chan live.Message, 100)

	go b.stream.Run(ctx, msgChan)

	for {
		if err := b.flushBatch(ctx); err != nil {
			log.Fatal().Err(err).Msg("无法处理已完成的盲盒数据批次")
		}

		select {
		case <-ctx.Done():
			return
		case msg := <-msgChan:
			b.handleMessage(msg)
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

	st := b.curBatch[sendGift.UID][sendGift.BlindGift.OriginalGiftID]
	st.TotalNum += sendGift.Num
	st.TotalOriginalPrice += sendGift.BlindGift.OriginalGiftPrice * sendGift.Num
	st.TotalPrice += sendGift.Price * sendGift.Num
	st.LastUpdateTime = time.Now()

	// Populate the box names lazily
	if _, ok := b.boxNames[sendGift.BlindGift.OriginalGiftID]; !ok {
		b.boxNames[sendGift.BlindGift.OriginalGiftID] = sendGift.BlindGift.OriginalGiftName
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
