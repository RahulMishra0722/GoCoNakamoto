package Addrman

import (
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"go.etcd.io/bbolt"
)

const PeerBucket = "Peers"

type Addrman struct {
	db   *bbolt.DB
	rwMu sync.RWMutex
}

var (
	once     sync.Once
	instance *Addrman
)

func GetAddrMan(path string) (*Addrman, error) {
	var initErr error

	once.Do(func() {
		options := &bbolt.Options{
			NoSync: false,
			// Wait up to 5s for the file lock if another process holds it.
			// This was `Timeout: 1` — 1 NANOSECOND — so any momentary
			// lock contention failed instantly with a timeout error.
			Timeout: 5 * time.Second,
		}

		db, err := bbolt.Open(path, 0666, options)
		if err != nil {
			initErr = fmt.Errorf("failed to open database: %w", err)
			return
		}

		// Initialize the Peers bucket
		if err := db.Update(func(tx *bbolt.Tx) error {
			_, err := tx.CreateBucketIfNotExists([]byte(PeerBucket))
			return err
		}); err != nil {
			initErr = fmt.Errorf("failed to create peers bucket: %w", err)
			db.Close()
			return
		}

		instance = &Addrman{
			db:   db,
			rwMu: sync.RWMutex{},
		}
	})

	if initErr != nil {
		return nil, initErr
	}

	return instance, nil
}

func (am *Addrman) Init(path string) (*Addrman, error) {
	return GetAddrMan(path)
}

func (am *Addrman) SaveAddr(addresses []string) error {
	if len(addresses) == 0 {
		fmt.Println("exiting ...")
		return nil
	}

	am.rwMu.Lock()
	defer am.rwMu.Unlock()

	return am.db.Batch(func(tx *bbolt.Tx) error {
		bucket := tx.Bucket([]byte(PeerBucket))
		if bucket == nil {
			return fmt.Errorf("peers bucket not found")
		}

		for _, addr := range addresses {
			if addr == "" {
				continue
			}

			if !am.DoesAddrAlreadyExist(addr) {
				data, err := json.Marshal(addr)
				if err != nil {
					return fmt.Errorf("failed to serialize address %s: %w", addr, err)
				}

				if err := bucket.Put([]byte(addr), data); err != nil {
					return fmt.Errorf("failed to store address %s: %w", addr, err)
				}
			}
		}
		return nil
	})
}

func (am *Addrman) GetAddr() ([]string, error) {
	am.rwMu.RLock()
	defer am.rwMu.RUnlock()

	var peers []string

	err := am.db.View(func(tx *bbolt.Tx) error {
		bucket := tx.Bucket([]byte(PeerBucket))
		if bucket == nil {
			return fmt.Errorf("peers bucket not found")
		}

		return bucket.ForEach(func(k, v []byte) error {
			peers = append(peers, string(k))
			return nil
		})
	})

	if err != nil {
		return nil, fmt.Errorf("failed to retrieve addresses: %w", err)
	}

	return peers, nil
}

func (am *Addrman) DoesAddrAlreadyExist(addr string) bool {
	if addr == "" {
		return false
	}

	var exists bool
	am.db.View(func(tx *bbolt.Tx) error {
		bucket := tx.Bucket([]byte(PeerBucket))
		if bucket == nil {
			return nil
		}
		exists = bucket.Get([]byte(addr)) != nil
		return nil
	})
	return exists
}

func (am *Addrman) RemoveAddr(addr string) error {
	if addr == "" {
		return fmt.Errorf("empty address provided")
	}

	am.rwMu.Lock()
	defer am.rwMu.Unlock()

	return am.db.Update(func(tx *bbolt.Tx) error {
		bucket := tx.Bucket([]byte(PeerBucket))
		if bucket == nil {
			return fmt.Errorf("peers bucket not found")
		}
		return bucket.Delete([]byte(addr))
	})
}

func (am *Addrman) GetAddrCount() (int, error) {
	am.rwMu.RLock()
	defer am.rwMu.RUnlock()

	var count int
	err := am.db.View(func(tx *bbolt.Tx) error {
		bucket := tx.Bucket([]byte(PeerBucket))
		if bucket == nil {
			return fmt.Errorf("peers bucket not found")
		}

		count = bucket.Stats().KeyN
		return nil
	})

	return count, err
}

func (am *Addrman) Close() error {
	am.rwMu.Lock()
	defer am.rwMu.Unlock()

	if am.db != nil {
		return am.db.Close()
	}
	return nil
}
