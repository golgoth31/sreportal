package dns_test

import (
	"reflect"
	"testing"

	dns "github.com/golgoth31/sreportal/internal/domain/dns"
)

func TestSplitGroups(t *testing.T) {
	cases := []struct {
		in   string
		want []string
	}{
		{"", nil},
		{"   ", nil},
		{",, ,", nil},
		{"a", []string{"a"}},
		{"a,b , c ", []string{"a", "b", "c"}},
	}
	for _, tc := range cases {
		if got := dns.SplitGroups(tc.in); !reflect.DeepEqual(got, tc.want) {
			t.Errorf("SplitGroups(%q) = %v, want %v", tc.in, got, tc.want)
		}
	}
}
