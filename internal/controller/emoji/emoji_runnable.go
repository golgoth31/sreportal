// Package emoji provides a manager.Runnable that periodically fetches custom
// emojis from Slack and pushes them to the emoji ReadStore.
package emoji

import (
	"context"
	"time"

	"sigs.k8s.io/controller-runtime/pkg/manager"

	domainemoji "github.com/golgoth31/sreportal/internal/domain/emoji"
	"github.com/golgoth31/sreportal/internal/log"
)

// EmojiSource fetches custom emojis from an external provider.
type EmojiSource interface {
	GetCustomEmojis(ctx context.Context) (map[string]string, error)
}

// Compile-time interface check.
var _ manager.Runnable = (*EmojiRunnable)(nil)

// EmojiRunnable fetches custom emojis from an external source at startup and on a periodic interval.
type EmojiRunnable struct {
	source   EmojiSource
	writer   domainemoji.EmojiWriter
	interval time.Duration
}

// NewEmojiRunnable creates a new EmojiRunnable.
func NewEmojiRunnable(source EmojiSource, writer domainemoji.EmojiWriter, interval time.Duration) *EmojiRunnable {
	return &EmojiRunnable{
		source:   source,
		writer:   writer,
		interval: interval,
	}
}

// Start implements manager.Runnable to run periodic emoji fetching.
//
// Error propagation: fetch errors are logged but not returned to the manager.
// A transient Slack API failure should not stop the operator — the next tick
// will retry automatically.
func (r *EmojiRunnable) Start(ctx context.Context) error {
	logger := log.Default().WithName("emoji")

	if err := r.fetch(ctx); err != nil {
		logger.Error(err, "initial emoji fetch failed")
	}

	ticker := time.NewTicker(r.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			logger.Info("stopping emoji fetcher")
			return nil
		case <-ticker.C:
			if err := r.fetch(ctx); err != nil {
				logger.Error(err, "periodic emoji fetch failed")
			}
		}
	}
}

func (r *EmojiRunnable) fetch(ctx context.Context) error {
	emojis, err := r.source.GetCustomEmojis(ctx)
	if err != nil {
		return err
	}
	return r.writer.ReplaceAll(ctx, emojis)
}
