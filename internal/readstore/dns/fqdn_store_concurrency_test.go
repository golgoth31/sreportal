package dns

import (
	"context"
	"fmt"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"

	domaindns "github.com/golgoth31/sreportal/internal/domain/dns"
)

func TestFQDNStore_ConcurrentReplaceAndDelete(t *testing.T) {
	ctx := context.Background()
	s := NewFQDNStore()

	var wg sync.WaitGroup
	for w := 0; w < 8; w++ {
		wg.Add(1)
		go func(w int) {
			defer wg.Done()
			for i := 0; i < 200; i++ {
				key := fmt.Sprintf("ns/rec-%d", (i+w)%16)
				portal := fmt.Sprintf("p%d", i%4)
				if i%5 == 0 {
					_ = s.Delete(ctx, key)
					continue
				}
				_ = s.Replace(ctx, key, portal, []domaindns.FQDNView{
					{Name: fmt.Sprintf("fqdn-%d.example.com", i%32), RecordType: "A", Targets: []string{"1.1.1.1"}},
				})
			}
		}(w)
	}
	wg.Wait()

	// Invariants:
	//   1) every FQDN in s.fqdns has non-empty Portals
	//   2) every (portal, key) in s.byPortal points to an existing fqdn
	s.mu.RLock()
	defer s.mu.RUnlock()
	for k, v := range s.fqdns {
		assert.NotEmpty(t, v.Portals, "fqdn %v has empty Portals", k)
	}
	for p, set := range s.byPortal {
		for k := range set {
			_, ok := s.fqdns[k]
			assert.Truef(t, ok, "byPortal[%s] references missing fqdn %v", p, k)
		}
	}
}
