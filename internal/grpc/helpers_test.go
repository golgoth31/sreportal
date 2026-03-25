package grpc_test

import (
	"testing"

	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/runtime"

	sreportalv1alpha1 "github.com/golgoth31/sreportal/api/v1alpha1"
)

func newScheme(t *testing.T) *runtime.Scheme {
	t.Helper()
	s := runtime.NewScheme()
	require.NoError(t, sreportalv1alpha1.AddToScheme(s))
	return s
}
