package emoji_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	emojistore "github.com/golgoth31/sreportal/internal/readstore/emoji"
)

func TestEmojiStore_ReplaceAll_ThenAll(t *testing.T) {
	store := emojistore.NewEmojiStore()
	ctx := context.Background()

	input := map[string]string{
		"rabbitmq": "https://emoji.slack.com/rabbitmq.png",
		"golang":   "https://emoji.slack.com/golang.png",
	}

	err := store.ReplaceAll(ctx, input)
	require.NoError(t, err)

	got, err := store.All(ctx)
	require.NoError(t, err)
	assert.Equal(t, input, got)
}

func TestEmojiStore_All_EmptyStore(t *testing.T) {
	store := emojistore.NewEmojiStore()

	got, err := store.All(context.Background())
	require.NoError(t, err)
	assert.Empty(t, got)
}

func TestEmojiStore_ReplaceAll_OverwritesPrevious(t *testing.T) {
	store := emojistore.NewEmojiStore()
	ctx := context.Background()

	_ = store.ReplaceAll(ctx, map[string]string{"old": "https://old.png"})
	_ = store.ReplaceAll(ctx, map[string]string{"new": "https://new.png"})

	got, err := store.All(ctx)
	require.NoError(t, err)
	assert.Equal(t, map[string]string{"new": "https://new.png"}, got)
}

func TestEmojiStore_Subscribe_NotifiesOnReplace(t *testing.T) {
	store := emojistore.NewEmojiStore()
	ch := store.Subscribe()

	_ = store.ReplaceAll(context.Background(), map[string]string{"x": "https://x.png"})

	select {
	case <-ch:
		// Expected: channel closed after mutation
	default:
		t.Fatal("expected subscribe channel to be closed after ReplaceAll")
	}
}
