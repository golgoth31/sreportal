package emoji_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	emojictrl "github.com/golgoth31/sreportal/internal/controller/emoji"
	emojistore "github.com/golgoth31/sreportal/internal/readstore/emoji"
	"github.com/golgoth31/sreportal/internal/slackclient"
)

func TestEmojiRunnable_FetchesAtStartup(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"ok": true,
			"emoji": map[string]string{
				"rabbitmq": "https://emoji.slack.com/rabbitmq.png",
			},
		})
	}))
	defer srv.Close()

	store := emojistore.NewEmojiStore()
	client := slackclient.NewClient("test-token", slackclient.WithBaseURL(srv.URL))
	runnable := emojictrl.NewEmojiRunnable(client, store, 24*time.Hour)

	ctx, cancel := context.WithCancel(context.Background())

	done := make(chan error, 1)
	go func() {
		done <- runnable.Start(ctx)
	}()

	// Wait for the store to be populated
	require.Eventually(t, func() bool {
		emojis, _ := store.All(context.Background())
		return len(emojis) > 0
	}, 2*time.Second, 10*time.Millisecond)

	emojis, err := store.All(context.Background())
	require.NoError(t, err)
	assert.Equal(t, "https://emoji.slack.com/rabbitmq.png", emojis["rabbitmq"])

	cancel()
	require.NoError(t, <-done)
}

func TestEmojiRunnable_ContinuesOnFetchError(t *testing.T) {
	callCount := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		callCount++
		if callCount == 1 {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"ok":    true,
			"emoji": map[string]string{"ok": "https://ok.png"},
		})
	}))
	defer srv.Close()

	store := emojistore.NewEmojiStore()
	client := slackclient.NewClient("test-token", slackclient.WithBaseURL(srv.URL))
	// Short interval to trigger a second fetch quickly
	runnable := emojictrl.NewEmojiRunnable(client, store, 50*time.Millisecond)

	ctx, cancel := context.WithCancel(context.Background())

	done := make(chan error, 1)
	go func() {
		done <- runnable.Start(ctx)
	}()

	// First fetch fails, second should succeed after ticker
	require.Eventually(t, func() bool {
		emojis, _ := store.All(context.Background())
		return len(emojis) > 0
	}, 2*time.Second, 10*time.Millisecond)

	cancel()
	require.NoError(t, <-done)
}
