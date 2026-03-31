package emoji

import "context"

// EmojiReader provides read access to the custom emoji mapping.
type EmojiReader interface {
	// All returns all custom emojis as a map of shortcode to image URL.
	All(ctx context.Context) (map[string]string, error)
	// Subscribe returns a channel that is closed on the next mutation.
	Subscribe() <-chan struct{}
}
