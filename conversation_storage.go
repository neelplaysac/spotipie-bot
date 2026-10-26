package main

import (
	"sync"
	"time"

	"github.com/PaulSonOfLars/gotgbot/v2/ext"
	"github.com/PaulSonOfLars/gotgbot/v2/ext/handlers/conversation"
)

// expiringStorage wraps conversation state with a TTL, mirroring
// python-telegram-bot's conversation_timeout=60: an idle conversation
// expires and the user can start over.
type expiringStorage struct {
	mu      sync.Mutex
	entries map[string]expEntry
	ttl     time.Duration
}

type expEntry struct {
	state   conversation.State
	updated time.Time
}

func newExpiringStorage(ttl time.Duration) *expiringStorage {
	return &expiringStorage{
		entries: map[string]expEntry{},
		ttl:     ttl,
	}
}

func keyFor(ctx *ext.Context) (string, error) {
	return conversation.StateKey(ctx, conversation.KeyStrategySenderAndChat)
}

func (s *expiringStorage) Get(ctx *ext.Context) (*conversation.State, error) {
	key, err := keyFor(ctx)
	if err != nil {
		return nil, err
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	e, ok := s.entries[key]
	if !ok {
		return nil, conversation.ErrKeyNotFound
	}
	if time.Since(e.updated) > s.ttl {
		delete(s.entries, key)
		return nil, conversation.ErrKeyNotFound
	}
	return &e.state, nil
}

func (s *expiringStorage) Set(ctx *ext.Context, state conversation.State) error {
	key, err := keyFor(ctx)
	if err != nil {
		return err
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	s.entries[key] = expEntry{state: state, updated: time.Now()}
	return nil
}

func (s *expiringStorage) Delete(ctx *ext.Context) error {
	key, err := keyFor(ctx)
	if err != nil {
		return err
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	delete(s.entries, key)
	return nil
}
