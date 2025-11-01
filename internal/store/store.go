// This package contains the storage layer for the application.

package store

import (
	"context"
	"time"
)

// A transfer interface for box statistics to be used to interact with the store.
// For Get operation, the store will unmarshal the data directly into the pointer returned by GetBoxStatistics.
// Conversely, for set operation, the store will marshal the pointer returned by GetBoxStatistics into the data.
type BoxStatisticsTransfer interface {
	Key() []byte
	GetBoxStatistics() *BoxStatistics
}

type NotFoundBehavior int

const (
	NotFoundBehaviorError NotFoundBehavior = iota
	NotFoundBehaviorSkip
)

// Storage layer interface for boxtroll
type Store interface {
	Close() error
	// Get user info by UID.
	GetUser(ctx context.Context, uid int64) (*User, error)
	// Set user info by UID.
	SetUser(ctx context.Context, uid int64, user *User) error
	// Get room info by Room ID.
	GetRoom(ctx context.Context, roomID int64) (*Room, error)
	// Set room info by Room ID.
	SetRoom(ctx context.Context, roomID int64, room *Room) error
	// Return a key representation for the given roomID, uid, boxID.
	BoxStatisticsKey(roomID int64, uid int64, boxID int64) []byte
	// Batch get box statistics.
	GetBoxStatistics(ctx context.Context, transfers []BoxStatisticsTransfer, notFoundBehavior NotFoundBehavior) error
	// Set box statistics.
	SetBoxStatistics(ctx context.Context, transfers []BoxStatisticsTransfer) error
}

// Statistics for a single <roomID, uid, boxID>, meaning,
// user UID's history of sending box boxID in room roomID.
// NOTE: Do NOT add JSON struct tag to this struct for backward compabilitity
type BoxStatistics struct {
	TotalNum           int64     // 盲盒总数量
	TotalOriginalPrice int64     // 盲盒总原价
	TotalPrice         int64     // 盲盒爆出礼物总价值
	LastUpdateTime     time.Time // 最后一次更新时间
}

func (b *BoxStatistics) Reset() {
	*b = BoxStatistics{}
}

func (b *BoxStatistics) Merge(other BoxStatistics) {
	b.TotalNum += other.TotalNum
	b.TotalOriginalPrice += other.TotalOriginalPrice
	b.TotalPrice += other.TotalPrice
	b.LastUpdateTime = other.LastUpdateTime
}

// Metadata for a single user. Keyed by UID.
type User struct {
	MID  int64  `json:"mid"`  // Use ID
	Name string `json:"name"` // User name
	Face string `json:"face"` // User avatar URL
}

// Metadata for a single live room. Keyed by Room ID.
type Room struct {
	RoomID int64 `json:"room_id"` // Room ID
	Gifts  []*Gift
}

// Metadata for a kind of gift.
type Gift struct {
	GiftID           int64             `json:"gift_id"`            // Gift ID
	Name             string            `json:"name"`               // Gift name
	Price            int64             `json:"price"`              // Gift price
	CoinType         string            `json:"coin_type"`          // Coin type (e.g., "gold" -> 金瓜子)
	ImgURL           string            `json:"img_url"`            // Gift image URL
	BlindBoxOutcomes []BlindBoxOutcome `json:"blind_box_outcomes"` // Outcomes of the blind box if this gift is a blind box
}

// Metadata for a single outcome of a blind box.
type BlindBoxOutcome struct {
	GiftID      int64   `json:"gift_id"`     // Gift ID
	Price       int64   `json:"price"`       // Price of the outcome
	Name        string  `json:"name"`        // Name of the outcome
	ImgURL      string  `json:"img_url"`     // Image URL of the outcome
	Probability float64 `json:"probability"` // Probability of the outcome
}
