package boxtroll

import (
	"context"
	"fmt"
	"sync"

	"github.com/YangchenYe323/boxtroll/internal/store"
)

// A boxtroll store implementation. It is built on top of a persistency layer and provide
// aggressive read caching of all the data used by boxtroll.
//
// It only operates on a single live room at a time, and fetches all the data from the persister upon initialization.
// During its lifetime, it serves all the read data from in-memory caches and persists write to persister and update in memory caches.
//
// We cannot directly use badger's memtable, e.g., for cache because memtable caches recent write but not read.
type boxtrollStore struct {
	persister store.Store
	roomID    int64

	userCacheMu sync.RWMutex
	userCache   map[int64]*store.User

	roomCacheMu sync.RWMutex
	roomCache   *store.Room

	boxStatisticsCacheMu sync.RWMutex
	boxStatisticsCache   map[string]*store.BoxStatistics
}

var _ store.Store = &boxtrollStore{}

func newBoxtrollStore(ctx context.Context, persister store.Store, roomID int64) (*boxtrollStore, error) {
	room, err := persister.GetRoom(ctx, roomID)
	if err != nil {
		// Room is not initialized, should not happen
		return nil, fmt.Errorf("failed to get room: %w", err)
	}

	userIDs, err := persister.ListAllUserIDs(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to list all user IDs: %w", err)
	}
	userCache := make(map[int64]*store.User)
	for _, userID := range userIDs {
		user, err := persister.GetUser(ctx, userID)
		if err != nil {
			return nil, fmt.Errorf("failed to get user: %w", err)
		}
		userCache[userID] = user
	}

	boxStatistics, err := persister.ListAllBoxStatistics(ctx, roomID)
	if err != nil {
		return nil, fmt.Errorf("failed to list all box statistics: %w", err)
	}

	return &boxtrollStore{
		persister:          persister,
		roomID:             roomID,
		roomCache:          room,
		userCache:          userCache,
		boxStatisticsCache: boxStatistics,
	}, nil
}

func (s *boxtrollStore) Close() error {
	return s.persister.Close()
}

func (s *boxtrollStore) ListAllUserIDs(ctx context.Context) ([]int64, error) {
	s.userCacheMu.RLock()
	defer s.userCacheMu.RUnlock()

	var userIDs = make([]int64, 0, len(s.userCache))
	for userID := range s.userCache {
		userIDs = append(userIDs, userID)
	}
	return userIDs, nil
}

func (s *boxtrollStore) GetUser(ctx context.Context, uid int64) (*store.User, error) {
	s.userCacheMu.RLock()
	defer s.userCacheMu.RUnlock()

	user, ok := s.userCache[uid]
	if !ok {
		return nil, store.ErrNotFound
	}

	return user, nil
}

func (s *boxtrollStore) SetUser(ctx context.Context, uid int64, user *store.User) error {
	if err := s.persister.SetUser(ctx, uid, user); err != nil {
		return err
	}

	s.userCacheMu.Lock()
	defer s.userCacheMu.Unlock()
	s.userCache[uid] = user

	return nil
}

func (s *boxtrollStore) GetRoom(ctx context.Context, roomID int64) (*store.Room, error) {
	if roomID != s.roomID {
		panic("boxtrollStore.GetRoom: roomID mismatch")
	}

	s.roomCacheMu.RLock()
	defer s.roomCacheMu.RUnlock()

	return s.roomCache, nil
}

func (s *boxtrollStore) SetRoom(ctx context.Context, roomID int64, room *store.Room) error {
	// Not implemented, do not use
	panic("boxtrollStore.SetRoom is NOT implemented")
}

func (s *boxtrollStore) BoxStatisticsKey(roomID int64, uid int64, boxID int64) []byte {
	return s.persister.BoxStatisticsKey(roomID, uid, boxID)
}

func (s *boxtrollStore) GetBoxStatistics(ctx context.Context, transfers []store.BoxStatisticsTransfer, notFoundBehavior store.NotFoundBehavior) error {
	s.boxStatisticsCacheMu.RLock()
	defer s.boxStatisticsCacheMu.RUnlock()

	for _, transfer := range transfers {
		st, ok := s.boxStatisticsCache[string(transfer.Key())]
		if !ok {
			switch notFoundBehavior {
			case store.NotFoundBehaviorError:
				return fmt.Errorf("%w: box statistics not found: %s", store.ErrNotFound, string(transfer.Key()))
			case store.NotFoundBehaviorSkip:
				continue
			}
		}
		// Copy it over
		*transfer.GetBoxStatistics() = *st
	}

	return nil
}

func (s *boxtrollStore) SetBoxStatistics(ctx context.Context, transfers []store.BoxStatisticsTransfer) error {
	if err := s.persister.SetBoxStatistics(ctx, transfers); err != nil {
		return err
	}

	s.boxStatisticsCacheMu.Lock()
	defer s.boxStatisticsCacheMu.Unlock()
	for _, transfer := range transfers {
		s.boxStatisticsCache[string(transfer.Key())] = transfer.GetBoxStatistics()
	}

	return nil
}

func (s *boxtrollStore) ListAllBoxSenderUserIDs(ctx context.Context, roomID int64) ([]int64, error) {
	// Not implemented, do not use
	panic("boxtrollStore.ListAllBoxSenderUserIDs is NOT implemented")
}

func (s *boxtrollStore) ListAllBoxStatistics(ctx context.Context, roomID int64) (map[string]*store.BoxStatistics, error) {
	if roomID != s.roomID {
		panic("boxtrollStore.ListAllBoxStatistics: roomID mismatch")
	}

	s.boxStatisticsCacheMu.RLock()
	defer s.boxStatisticsCacheMu.RUnlock()

	// Copy it over
	boxStatistics := make(map[string]*store.BoxStatistics)
	for key, st := range s.boxStatisticsCache {
		var copySt = *st
		boxStatistics[key] = &copySt
	}

	return boxStatistics, nil
}
