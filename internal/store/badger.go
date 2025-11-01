package store

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/dgraph-io/badger/v4"
	"github.com/rs/zerolog/log"
)

type badgerStore struct {
	b *badger.DB
}

var _ Store = &badgerStore{}

func NewBadger(dbPath string) (Store, error) {
	if dbPath == "" {
		return nil, errors.New("db path is empty")
	}

	opts := badger.DefaultOptions(dbPath)
	// Use zerolog as the logger
	opts.Logger = &badgerLoggerAdapter{}

	db, err := badger.Open(opts)
	if err != nil {
		return nil, err
	}

	return &badgerStore{b: db}, nil
}

func (b *badgerStore) Close() error {
	return b.b.Close()
}

func (b *badgerStore) GetUser(ctx context.Context, uid int64) (*User, error) {
	key := fmt.Appendf(nil, "user/%d", uid)

	var user User
	if err := b.b.View(func(txn *badger.Txn) error {
		item, err := txn.Get(key)
		if err != nil {
			return err
		}
		return item.Value(func(val []byte) error {
			return json.Unmarshal(val, &user)
		})
	}); err != nil {
		return nil, err
	}

	return &user, nil
}

func (b *badgerStore) SetUser(ctx context.Context, uid int64, user *User) error {
	key := fmt.Appendf(nil, "user/%d", uid)
	return b.b.Update(func(txn *badger.Txn) error {
		bytes, err := json.Marshal(user)
		if err != nil {
			return err
		}
		return txn.Set(key, bytes)
	})
}

func (b *badgerStore) GetRoom(ctx context.Context, roomID int64) (*Room, error) {
	key := fmt.Appendf(nil, "room/%d", roomID)
	var room Room
	if err := b.b.View(func(txn *badger.Txn) error {
		item, err := txn.Get(key)
		if err != nil {
			return err
		}
		return item.Value(func(val []byte) error {
			return json.Unmarshal(val, &room)
		})
	}); err != nil {
		return nil, err
	}
	return &room, nil
}

func (b *badgerStore) SetRoom(ctx context.Context, roomID int64, room *Room) error {
	key := fmt.Appendf(nil, "room/%d", roomID)
	return b.b.Update(func(txn *badger.Txn) error {
		bytes, err := json.Marshal(room)
		if err != nil {
			return err
		}
		return txn.Set(key, bytes)
	})
}

func (b *badgerStore) BoxStatisticsKey(roomID int64, uid int64, boxID int64) []byte {
	return fmt.Appendf(nil, "%d/%d/%d", roomID, uid, boxID)
}

func (b *badgerStore) GetBoxStatistics(ctx context.Context, transfers []BoxStatisticsTransfer, notFoundBehavior NotFoundBehavior) error {
	return b.b.View(func(txn *badger.Txn) error {
		for _, transfer := range transfers {
			key := transfer.Key()

			item, err := txn.Get(key)

			if errors.Is(err, badger.ErrKeyNotFound) {
				switch notFoundBehavior {
				case NotFoundBehaviorError:
					return fmt.Errorf("box statistics not found: %s", string(key))
				case NotFoundBehaviorSkip:
					continue
				}
			}

			if err != nil {
				return fmt.Errorf("failed to get box statistics: %s", string(key))
			}

			if err := item.Value(func(val []byte) error {
				return json.Unmarshal(val, transfer.GetBoxStatistics())
			}); err != nil {
				return fmt.Errorf("failed to unmarshal box statistics: %s", string(key))
			}
		}
		return nil
	})
}

func (b *badgerStore) SetBoxStatistics(ctx context.Context, transfers []BoxStatisticsTransfer) error {
	return b.b.Update(func(txn *badger.Txn) error {
		for _, transfer := range transfers {
			bytes, err := json.Marshal(transfer.GetBoxStatistics())
			if err != nil {
				return fmt.Errorf("failed to marshal box statistics: %s", string(transfer.Key()))
			}

			if err := txn.Set(transfer.Key(), bytes); err != nil {
				return fmt.Errorf("failed to set box statistics: %s", string(transfer.Key()))
			}
		}
		return nil
	})
}

type badgerLoggerAdapter struct{}

func (l *badgerLoggerAdapter) Errorf(format string, v ...interface{}) {
	log.Error().Msgf(format, v...)
}

func (l *badgerLoggerAdapter) Warningf(format string, v ...interface{}) {
	log.Warn().Msgf(format, v...)
}

func (l *badgerLoggerAdapter) Infof(format string, v ...interface{}) {
	log.Info().Msgf(format, v...)
}

func (l *badgerLoggerAdapter) Debugf(format string, v ...interface{}) {
	log.Debug().Msgf(format, v...)
}
