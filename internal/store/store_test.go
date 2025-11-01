package store_test

import (
	"context"
	"testing"
	"time"

	"github.com/YangchenYe323/boxtroll/internal/store"
)

type testBoxStatisticsTransfer struct {
	key []byte
	st  store.BoxStatistics
}

func (t *testBoxStatisticsTransfer) Key() []byte {
	return t.key
}

func (t *testBoxStatisticsTransfer) GetBoxStatistics() *store.BoxStatistics {
	return &t.st
}

func TestBoxStatisticsOperations(t *testing.T) {
	badgerStore, err := store.NewBadger(t.TempDir())
	if err != nil {
		t.Fatalf("failed to create badger store: %v", err)
	}

	// This is necessary or tempdir cleanup fails on windows
	defer badgerStore.Close()

	transfers := []store.BoxStatisticsTransfer{
		&testBoxStatisticsTransfer{
			key: badgerStore.BoxStatisticsKey(1, 1, 1),
			st: store.BoxStatistics{
				TotalNum:           100,
				TotalOriginalPrice: 1000,
				TotalPrice:         10000,
				LastUpdateTime:     time.Now(),
			},
		},
		&testBoxStatisticsTransfer{
			key: badgerStore.BoxStatisticsKey(1, 1, 2),
			st: store.BoxStatistics{
				TotalNum:           200,
				TotalOriginalPrice: 2000,
				TotalPrice:         20000,
				LastUpdateTime:     time.Now(),
			},
		},
	}

	if err := badgerStore.SetBoxStatistics(context.Background(), transfers); err != nil {
		t.Fatalf("failed to batch transfer box statistics: %v", err)
	}

	actial := []store.BoxStatisticsTransfer{
		&testBoxStatisticsTransfer{
			key: badgerStore.BoxStatisticsKey(1, 1, 1),
			st:  store.BoxStatistics{},
		},
		&testBoxStatisticsTransfer{
			key: badgerStore.BoxStatisticsKey(1, 1, 2),
			st:  store.BoxStatistics{},
		},
	}

	if err := badgerStore.GetBoxStatistics(context.Background(), actial, store.NotFoundBehaviorError); err != nil {
		t.Fatalf("failed to batch transfer box statistics: %v", err)
	}

	for i := range len(transfers) {
		expected := transfers[i].(*testBoxStatisticsTransfer)
		expectedSt := expected.GetBoxStatistics()
		actual := actial[i].(*testBoxStatisticsTransfer)
		actualSt := actual.GetBoxStatistics()

		if expectedSt.TotalNum != actualSt.TotalNum {
			t.Fatalf("expected total num %d, got %d", expectedSt.TotalNum, actualSt.TotalNum)
		}
		if expectedSt.TotalOriginalPrice != actualSt.TotalOriginalPrice {
			t.Fatalf("expected total original price %d, got %d", expectedSt.TotalOriginalPrice, actualSt.TotalOriginalPrice)
		}
		if expectedSt.TotalPrice != actualSt.TotalPrice {
			t.Fatalf("expected total price %d, got %d", expectedSt.TotalPrice, actualSt.TotalPrice)
		}
		if !expectedSt.LastUpdateTime.Equal(actualSt.LastUpdateTime) {
			t.Fatalf("expected last update time %v, got %v", expectedSt.LastUpdateTime, actualSt.LastUpdateTime)
		}
	}
}

func TestUserOperations(t *testing.T) {
	badgerStore, err := store.NewBadger(t.TempDir())
	if err != nil {
		t.Fatalf("failed to create badger store: %v", err)
	}
	defer badgerStore.Close()

	user := &store.User{
		MID:  1,
		Name: "test",
		Face: "test",
	}

	if err := badgerStore.SetUser(context.Background(), 1, user); err != nil {
		t.Fatalf("failed to set user: %v", err)
	}

	actual, err := badgerStore.GetUser(context.Background(), 1)
	if err != nil {
		t.Fatalf("failed to get user: %v", err)
	}
	if actual.MID != user.MID {
		t.Fatalf("expected user MID %d, got %d", user.MID, actual.MID)
	}
	if actual.Name != user.Name {
		t.Fatalf("expected user name %s, got %s", user.Name, actual.Name)
	}
	if actual.Face != user.Face {
		t.Fatalf("expected user face %s, got %s", user.Face, actual.Face)
	}
}

func TestRoomOperations(t *testing.T) {
	badgerStore, err := store.NewBadger(t.TempDir())
	if err != nil {
		t.Fatalf("failed to create badger store: %v", err)
	}
	defer badgerStore.Close()

	room := &store.Room{
		RoomID: 1,
		Gifts: []*store.Gift{
			{
				GiftID:   1,
				Name:     "test",
				Price:    100,
				CoinType: "test",
				ImgURL:   "test",
			},
		},
	}

	if err := badgerStore.SetRoom(context.Background(), 1, room); err != nil {
		t.Fatalf("failed to set room: %v", err)
	}

	actual, err := badgerStore.GetRoom(context.Background(), 1)
	if err != nil {
		t.Fatalf("failed to get room: %v", err)
	}
	if actual.RoomID != room.RoomID {
		t.Fatalf("expected room ID %d, got %d", room.RoomID, actual.RoomID)
	}
	if len(actual.Gifts) != len(room.Gifts) {
		t.Fatalf("expected %d gifts, got %d", len(room.Gifts), len(actual.Gifts))
	}
	for i := range len(room.Gifts) {
		expected := room.Gifts[i]
		actual := actual.Gifts[i]
		if expected.GiftID != actual.GiftID {
			t.Fatalf("expected gift ID %d, got %d", expected.GiftID, actual.GiftID)
		}
		if expected.Name != actual.Name {
			t.Fatalf("expected gift name %s, got %s", expected.Name, actual.Name)
		}
	}
}
