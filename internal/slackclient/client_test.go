package slackclient_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/golgoth31/sreportal/internal/domain/emoji"
	"github.com/golgoth31/sreportal/internal/slackclient"
)

func TestGetCustomEmojis(t *testing.T) {
	cases := []struct {
		name       string
		handler    http.HandlerFunc
		wantEmojis map[string]string
		wantErr    error
	}{
		{
			name: "happy path with direct URLs",
			handler: jsonHandler(map[string]any{
				"ok": true,
				"emoji": map[string]string{
					"rabbitmq": "https://emoji.slack.com/rabbitmq.png",
					"golang":   "https://emoji.slack.com/golang.png",
				},
			}),
			wantEmojis: map[string]string{
				"rabbitmq": "https://emoji.slack.com/rabbitmq.png",
				"golang":   "https://emoji.slack.com/golang.png",
			},
		},
		{
			name: "resolves simple alias",
			handler: jsonHandler(map[string]any{
				"ok": true,
				"emoji": map[string]string{
					"rabbitmq":  "https://emoji.slack.com/rabbitmq.png",
					"rabbit_mq": "alias:rabbitmq",
				},
			}),
			wantEmojis: map[string]string{
				"rabbitmq":  "https://emoji.slack.com/rabbitmq.png",
				"rabbit_mq": "https://emoji.slack.com/rabbitmq.png",
			},
		},
		{
			name: "resolves chained aliases",
			handler: jsonHandler(map[string]any{
				"ok": true,
				"emoji": map[string]string{
					"original": "https://emoji.slack.com/original.png",
					"alias1":   "alias:original",
					"alias2":   "alias:alias1",
				},
			}),
			wantEmojis: map[string]string{
				"original": "https://emoji.slack.com/original.png",
				"alias1":   "https://emoji.slack.com/original.png",
				"alias2":   "https://emoji.slack.com/original.png",
			},
		},
		{
			name: "drops circular aliases",
			handler: jsonHandler(map[string]any{
				"ok": true,
				"emoji": map[string]string{
					"a":    "alias:b",
					"b":    "alias:a",
					"real": "https://emoji.slack.com/real.png",
				},
			}),
			wantEmojis: map[string]string{
				"real": "https://emoji.slack.com/real.png",
			},
		},
		{
			name: "drops self-referencing alias",
			handler: jsonHandler(map[string]any{
				"ok": true,
				"emoji": map[string]string{
					"loop": "alias:loop",
					"real": "https://emoji.slack.com/real.png",
				},
			}),
			wantEmojis: map[string]string{
				"real": "https://emoji.slack.com/real.png",
			},
		},
		{
			name: "drops alias to nonexistent target",
			handler: jsonHandler(map[string]any{
				"ok": true,
				"emoji": map[string]string{
					"orphan": "alias:does_not_exist",
					"real":   "https://emoji.slack.com/real.png",
				},
			}),
			wantEmojis: map[string]string{
				"real": "https://emoji.slack.com/real.png",
			},
		},
		{
			name: "slack API error",
			handler: jsonHandler(map[string]any{
				"ok":    false,
				"error": "invalid_auth",
			}),
			wantErr: emoji.ErrFetchEmojis,
		},
		{
			name: "empty emoji list",
			handler: jsonHandler(map[string]any{
				"ok":    true,
				"emoji": map[string]string{},
			}),
			wantEmojis: map[string]string{},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			srv := httptest.NewServer(tc.handler)
			defer srv.Close()

			client := slackclient.NewClient("test-token", slackclient.WithBaseURL(srv.URL))
			emojis, err := client.GetCustomEmojis(context.Background())

			if tc.wantErr != nil {
				require.ErrorIs(t, err, tc.wantErr)
				return
			}

			require.NoError(t, err)
			assert.Equal(t, tc.wantEmojis, emojis)
		})
	}
}

func TestGetCustomEmojis_HTTPError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	client := slackclient.NewClient("test-token", slackclient.WithBaseURL(srv.URL))
	_, err := client.GetCustomEmojis(context.Background())

	require.ErrorIs(t, err, emoji.ErrFetchEmojis)
}

func TestGetCustomEmojis_SetsAuthHeader(t *testing.T) {
	var gotAuth string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		_ = json.NewEncoder(w).Encode(map[string]any{"ok": true, "emoji": map[string]string{}})
	}))
	defer srv.Close()

	client := slackclient.NewClient("xoxb-my-token", slackclient.WithBaseURL(srv.URL))
	_, err := client.GetCustomEmojis(context.Background())

	require.NoError(t, err)
	assert.Equal(t, "Bearer xoxb-my-token", gotAuth)
}

func jsonHandler(body any) http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(body)
	}
}
