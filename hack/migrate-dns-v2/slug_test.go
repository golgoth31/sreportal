package main

import "testing"

func TestSlug(t *testing.T) {
	const defaultSlug = "default"
	cases := map[string]string{
		"":          defaultSlug,
		"Apps":      "apps",
		"My Group":  "my-group",
		"-leading":  "leading",
		"trailing-": "trailing",
		"___":       defaultSlug,
		"A B C":     "a-b-c",
	}
	for in, want := range cases {
		if got := slug(in); got != want {
			t.Errorf("slug(%q) = %q, want %q", in, got, want)
		}
	}
}
