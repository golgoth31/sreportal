// Package emoji contains pure domain types for custom emoji resolution.
package emoji

// CustomEmoji represents a custom emoji resolved to an image URL.
type CustomEmoji struct {
	Name     string
	ImageURL string
}
