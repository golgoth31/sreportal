// Package emoji provides an in-memory read store for custom emoji data.
package emoji

import (
	"context"

	domainemoji "github.com/golgoth31/sreportal/internal/domain/emoji"
	"github.com/golgoth31/sreportal/internal/readstore"
)

const storeKey = "slack"

// Compile-time interface checks.
var (
	_ domainemoji.EmojiReader = (*EmojiStore)(nil)
	_ domainemoji.EmojiWriter = (*EmojiStore)(nil)
)

// EmojiStore is an in-memory store for custom emoji mappings.
type EmojiStore struct {
	store *readstore.Store[domainemoji.CustomEmoji]
}

// NewEmojiStore creates a new empty EmojiStore.
func NewEmojiStore() *EmojiStore {
	return &EmojiStore{
		store: readstore.New[domainemoji.CustomEmoji](),
	}
}

// ReplaceAll atomically replaces all custom emojis.
func (s *EmojiStore) ReplaceAll(_ context.Context, emojis map[string]string) error {
	items := make([]domainemoji.CustomEmoji, 0, len(emojis))
	for name, url := range emojis {
		items = append(items, domainemoji.CustomEmoji{Name: name, ImageURL: url})
	}
	s.store.Replace(storeKey, items)
	return nil
}

// All returns all custom emojis as a map of shortcode to image URL.
func (s *EmojiStore) All(_ context.Context) (map[string]string, error) {
	items := s.store.All()
	m := make(map[string]string, len(items))
	for _, e := range items {
		m[e.Name] = e.ImageURL
	}
	return m, nil
}

// Subscribe returns a channel that is closed on the next mutation.
func (s *EmojiStore) Subscribe() <-chan struct{} {
	return s.store.Subscribe()
}
