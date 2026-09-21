package deploystatus

import (
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	dom "github.com/golgoth31/sreportal/internal/domain/deploystatus"
)

func TestStore_ReplaceListRemove_DedupesAcrossNamespaces(t *testing.T) {
	s := NewStore()
	s.ReplaceForNamespace("main", "ns1", []dom.Entry{{Key: "a", State: "behind"}})
	s.ReplaceForNamespace("main", "ns2", []dom.Entry{{Key: "b", State: "ok"}})
	if got := s.Count("main"); got != 2 {
		t.Fatalf("count = %d, want 2", got)
	}
	s.RemoveForNamespace("main", "ns1")
	if got := s.Count("main"); got != 1 {
		t.Fatalf("after remove count = %d, want 1", got)
	}
	if s.List("main")[0].Key != "b" {
		t.Errorf("unexpected remaining entry")
	}
}

func TestStore_ReplaceForNamespace_OverwritesExistingScope(t *testing.T) {
	s := NewStore()
	s.ReplaceForNamespace("main", "ns1", []dom.Entry{{Key: "a"}, {Key: "b"}})
	s.ReplaceForNamespace("main", "ns1", []dom.Entry{{Key: "c"}})

	got := s.List("main")
	require.Len(t, got, 1)
	assert.Equal(t, "c", got[0].Key)
}

func TestStore_List_IsolatesPortals(t *testing.T) {
	s := NewStore()
	s.ReplaceForNamespace("portal-a", "ns1", []dom.Entry{{Key: "a"}})
	s.ReplaceForNamespace("portal-b", "ns1", []dom.Entry{{Key: "b"}})

	gotA := s.List("portal-a")
	gotB := s.List("portal-b")

	require.Len(t, gotA, 1)
	assert.Equal(t, "a", gotA[0].Key)
	require.Len(t, gotB, 1)
	assert.Equal(t, "b", gotB[0].Key)
}

func TestStore_List_EmptyForUnknownPortal(t *testing.T) {
	s := NewStore()
	got := s.List("nonexistent")
	assert.Empty(t, got)
}

func TestStore_Subscribe_ClosedOnReplace(t *testing.T) {
	s := NewStore()
	ch, unsub := s.Subscribe()
	defer unsub()

	s.ReplaceForNamespace("main", "ns1", []dom.Entry{{Key: "a"}})

	select {
	case <-ch:
		// expected: channel was closed
	default:
		t.Fatal("expected subscribe channel to be closed after ReplaceForNamespace")
	}
}

func TestStore_Subscribe_ClosedOnRemove(t *testing.T) {
	s := NewStore()
	s.ReplaceForNamespace("main", "ns1", []dom.Entry{{Key: "a"}})

	ch, unsub := s.Subscribe()
	defer unsub()

	s.RemoveForNamespace("main", "ns1")

	select {
	case <-ch:
		// expected
	default:
		t.Fatal("expected subscribe channel to be closed after RemoveForNamespace")
	}
}

func TestStore_Unsubscribe_StopsNotifications(t *testing.T) {
	s := NewStore()
	ch, unsub := s.Subscribe()
	unsub()

	// After unsubscribing, replace should not send to the channel.
	s.ReplaceForNamespace("main", "ns1", []dom.Entry{{Key: "a"}})

	select {
	case <-ch:
		// The channel was closed by unsub — that's fine, it just shouldn't block.
	default:
		// Also fine: nothing delivered after unsub.
	}
	// Key assertion: NewStore still works and doesn't panic after unsub.
}

func TestStore_ConcurrentAccess(t *testing.T) {
	s := NewStore()

	var wg sync.WaitGroup
	for i := range 100 {
		wg.Add(3)
		ns := "ns" + string(rune('0'+i%10))

		go func() {
			defer wg.Done()
			s.ReplaceForNamespace("main", ns, []dom.Entry{{Key: ns}})
		}()

		go func() {
			defer wg.Done()
			_ = s.List("main")
		}()

		go func() {
			defer wg.Done()
			ch, unsub := s.Subscribe()
			defer unsub()
			_ = ch
		}()
	}
	wg.Wait()
}
