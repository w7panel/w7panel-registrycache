package logic

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"time"

	bolt "go.etcd.io/bbolt"
)

// CacheExpirationPersistence keeps database details outside the scheduler.
type CacheExpirationPersistence interface {
	Save(CacheExpirationTask) error
	Delete(CacheExpirationTask) error
	Snapshot() ([]CacheExpirationTask, error)
	Close() error
}

type CacheExpirationStore struct{ db *bolt.DB }

var expirationBucket = []byte("tasks")

func NewCacheExpirationStore(directory string) (*CacheExpirationStore, error) {
	if directory == "" {
		return nil, errors.New("cache expiration directory is empty")
	}
	if err := os.MkdirAll(directory, 0755); err != nil {
		return nil, err
	}
	db, err := bolt.Open(filepath.Join(directory, "expiration.db"), 0600, &bolt.Options{Timeout: 5 * time.Second})
	if err != nil {
		return nil, err
	}
	if err = db.Update(func(tx *bolt.Tx) error { _, err := tx.CreateBucketIfNotExists(expirationBucket); return err }); err != nil {
		_ = db.Close()
		return nil, err
	}
	return &CacheExpirationStore{db: db}, nil
}

func (s *CacheExpirationStore) Save(task CacheExpirationTask) error {
	if task.Host == "" || task.RegistryURL == "" || task.Repository == "" || task.SourceRepository == "" || task.Reference == "" || task.Digest == "" || task.CachedAt.IsZero() {
		return errors.New("incomplete cache expiration task")
	}
	content, err := json.Marshal(task)
	if err != nil {
		return err
	}
	return s.db.Update(func(tx *bolt.Tx) error { return tx.Bucket(expirationBucket).Put([]byte(task.key()), content) })
}

func (s *CacheExpirationStore) Delete(task CacheExpirationTask) error {
	return s.db.Update(func(tx *bolt.Tx) error { return tx.Bucket(expirationBucket).Delete([]byte(task.key())) })
}

func (s *CacheExpirationStore) Snapshot() ([]CacheExpirationTask, error) {
	tasks := []CacheExpirationTask{}
	err := s.db.View(func(tx *bolt.Tx) error {
		return tx.Bucket(expirationBucket).ForEach(func(_, data []byte) error {
			var task CacheExpirationTask
			if err := json.Unmarshal(data, &task); err != nil {
				return err
			}
			tasks = append(tasks, task)
			return nil
		})
	})
	return tasks, err
}

func (s *CacheExpirationStore) Close() error { return s.db.Close() }
