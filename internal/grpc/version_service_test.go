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

package grpc_test

import (
	"context"
	"testing"

	"connectrpc.com/connect"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	svcgrpc "github.com/golgoth31/sreportal/internal/grpc"
	versionv1 "github.com/golgoth31/sreportal/internal/grpc/gen/sreportal/v1"
	"github.com/golgoth31/sreportal/internal/version"
)

func TestGetVersion_ReturnsVersionInfo(t *testing.T) {
	svc := svcgrpc.NewVersionService()

	resp, err := svc.GetVersion(context.Background(), connect.NewRequest(&versionv1.GetVersionRequest{}))
	require.NoError(t, err)

	assert.Equal(t, version.Version(), resp.Msg.Version)
	assert.Equal(t, version.Commit(), resp.Msg.Commit)
	assert.Equal(t, version.Date(), resp.Msg.Date)
}
