package readstore_test

import (
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/golgoth31/sreportal/internal/readstore"
)

type item struct {
	Name string
}

func TestStore_Replace_StoresItems(t *testing.T) {
	s := readstore.New[item]()

	s.Replace("key1", []item{{Name: "a"}, {Name: "b"}})

	all := s.All()
	require.Len(t, all, 2)
	assert.Equal(t, "a", all[0].Name)
	assert.Equal(t, "b", all[1].Name)
}

func TestStore_Replace_OverwritesExistingKey(t *testing.T) {
	s := readstore.New[item]()

	s.Replace("key1", []item{{Name: "a"}})
	s.Replace("key1", []item{{Name: "b"}, {Name: "c"}})

	all := s.All()
	require.Len(t, all, 2)
	assert.Equal(t, "b", all[0].Name)
}

func TestStore_Delete_RemovesKey(t *testing.T) {
	s := readstore.New[item]()

	s.Replace("key1", []item{{Name: "a"}})
	s.Replace("key2", []item{{Name: "b"}})
	s.Delete("key1")

	all := s.All()
	require.Len(t, all, 1)
	assert.Equal(t, "b", all[0].Name)
}

func TestStore_Delete_NoopForMissingKey(t *testing.T) {
	s := readstore.New[item]()

	s.Delete("nonexistent") // must not panic

	assert.Empty(t, s.All())
}

func TestStore_All_ReturnsFlatSnapshot(t *testing.T) {
	s := readstore.New[item]()

	s.Replace("key1", []item{{Name: "a"}, {Name: "b"}})
	s.Replace("key2", []item{{Name: "c"}})

	all := s.All()
	require.Len(t, all, 3)
}

func TestStore_All_ReturnsEmptyWhenNoData(t *testing.T) {
	s := readstore.New[item]()

	all := s.All()
	require.Empty(t, all)
	require.NotNil(t, all) // must return non-nil empty slice
}

func TestStore_All_ReturnsCopy(t *testing.T) {
	s := readstore.New[item]()
	s.Replace("key1", []item{{Name: "a"}})

	snap1 := s.All()
	snap1[0].Name = "mutated"

	snap2 := s.All()
	assert.Equal(t, "a", snap2[0].Name, "mutation of returned slice must not affect store")
}

func TestStore_Subscribe_ClosedOnReplace(t *testing.T) {
	s := readstore.New[item]()

	ch := s.Subscribe()

	s.Replace("key1", []item{{Name: "a"}})

	select {
	case <-ch:
		// expected: channel was closed
	default:
		t.Fatal("expected subscribe channel to be closed after Replace")
	}
}

func TestStore_Subscribe_ClosedOnDelete(t *testing.T) {
	s := readstore.New[item]()
	s.Replace("key1", []item{{Name: "a"}})

	ch := s.Subscribe()

	s.Delete("key1")

	select {
	case <-ch:
		// expected
	default:
		t.Fatal("expected subscribe channel to be closed after Delete")
	}
}

func TestStore_Subscribe_NewChannelAfterNotification(t *testing.T) {
	s := readstore.New[item]()

	ch1 := s.Subscribe()
	s.Replace("key1", []item{{Name: "a"}})
	<-ch1

	ch2 := s.Subscribe()

	// ch2 must be a new open channel
	select {
	case <-ch2:
		t.Fatal("new subscribe channel should not be closed yet")
	default:
		// expected: still open
	}

	s.Replace("key1", []item{{Name: "b"}})
	<-ch2 // now it should be closed
}

func TestStore_ConcurrentAccess(t *testing.T) {
	s := readstore.New[item]()

	var wg sync.WaitGroup
	for i := range 100 {
		wg.Add(3)
		key := "key" + string(rune('0'+i%10))

		go func() {
			defer wg.Done()
			s.Replace(key, []item{{Name: key}})
		}()

		go func() {
			defer wg.Done()
			_ = s.All()
		}()

		go func() {
			defer wg.Done()
			_ = s.Subscribe()
		}()
	}
	wg.Wait()

	// If we got here without -race detector flagging anything, we're good
	assert.NotNil(t, s.All())
}
