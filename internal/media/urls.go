package media

import (
	"strconv"

	"github.com/nermline/Melotrack-Creator/internal/store"
)

// URL prefixes served by the API (see api.Router). All of them need a session.
const (
	SourcePrefix = "/files/source/"
	ThumbPrefix  = "/files/thumbs/"
	ClipPrefix   = "/files/clips/"
	ImagePrefix  = "/files/images/"
)

func urlFor(prefix, name string) string {
	if name == "" {
		return ""
	}
	return prefix + name
}

func ClipURL(name string) string  { return urlFor(ClipPrefix, name) }
func ImageURL(name string) string { return urlFor(ImagePrefix, name) }
func ThumbURL(name string) string { return urlFor(ThumbPrefix, name) }
func SourceURL(m *store.Media) string {
	if m.Status != store.MediaReady {
		return ""
	}
	// The version query busts the browser cache when the file is replaced.
	return urlFor(SourcePrefix, m.SourceFile) + "?v=" + strconv.Itoa(m.Version)
}

// AnswerImageURL is the picture shown with the answer: the uploaded image,
// or the YouTube thumbnail when there is none.
func AnswerImageURL(it *store.Item, m *store.Media) string {
	if it.ImageFile != "" {
		return ImageURL(it.ImageFile)
	}
	return ThumbURL(m.ThumbFile)
}
