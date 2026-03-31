/*
Copyright 2026.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package grpc

import (
	"context"

	"connectrpc.com/connect"

	domainemoji "github.com/golgoth31/sreportal/internal/domain/emoji"
	portalv1 "github.com/golgoth31/sreportal/internal/grpc/gen/sreportal/v1"
	"github.com/golgoth31/sreportal/internal/grpc/gen/sreportal/v1/sreportalv1connect"
)

// EmojiService implements the EmojiServiceHandler interface.
type EmojiService struct {
	sreportalv1connect.UnimplementedEmojiServiceHandler
	reader domainemoji.EmojiReader
}

// NewEmojiService creates a new EmojiService.
func NewEmojiService(reader domainemoji.EmojiReader) *EmojiService {
	return &EmojiService{reader: reader}
}

// ListCustomEmojis returns all custom emojis as a map of shortcode to image URL.
// Returns an empty map if no emojis are loaded (graceful degradation).
func (s *EmojiService) ListCustomEmojis(
	ctx context.Context,
	_ *connect.Request[portalv1.ListCustomEmojisRequest],
) (*connect.Response[portalv1.ListCustomEmojisResponse], error) {
	emojis, err := s.reader.All(ctx)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	return connect.NewResponse(&portalv1.ListCustomEmojisResponse{
		Emojis: emojis,
	}), nil
}
