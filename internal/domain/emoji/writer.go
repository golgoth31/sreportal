package emoji

import "context"

// EmojiWriter provides write access to the custom emoji store.
type EmojiWriter interface {
	// ReplaceAll atomically replaces all custom emojis.
	ReplaceAll(ctx context.Context, emojis map[string]string) error
}
