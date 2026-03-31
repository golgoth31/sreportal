package grpc_test

import (
	"context"
	"testing"

	"connectrpc.com/connect"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	grpchandler "github.com/golgoth31/sreportal/internal/grpc"
	portalv1 "github.com/golgoth31/sreportal/internal/grpc/gen/sreportal/v1"
	emojistore "github.com/golgoth31/sreportal/internal/readstore/emoji"
)

func TestEmojiService_ListCustomEmojis_WithData(t *testing.T) {
	store := emojistore.NewEmojiStore()
	_ = store.ReplaceAll(context.Background(), map[string]string{
		"rabbitmq": "https://emoji.slack.com/rabbitmq.png",
		"golang":   "https://emoji.slack.com/golang.png",
	})

	svc := grpchandler.NewEmojiService(store)
	resp, err := svc.ListCustomEmojis(context.Background(), connect.NewRequest(&portalv1.ListCustomEmojisRequest{}))

	require.NoError(t, err)
	assert.Equal(t, "https://emoji.slack.com/rabbitmq.png", resp.Msg.Emojis["rabbitmq"])
	assert.Equal(t, "https://emoji.slack.com/golang.png", resp.Msg.Emojis["golang"])
	assert.Len(t, resp.Msg.Emojis, 2)
}

func TestEmojiService_ListCustomEmojis_EmptyStore(t *testing.T) {
	store := emojistore.NewEmojiStore()
	svc := grpchandler.NewEmojiService(store)

	resp, err := svc.ListCustomEmojis(context.Background(), connect.NewRequest(&portalv1.ListCustomEmojisRequest{}))

	require.NoError(t, err)
	assert.Empty(t, resp.Msg.Emojis)
}
