package media

import "testing"

func TestExtractYouTubeID(t *testing.T) {
	cases := []struct {
		name, url, want string
	}{
		{"watch full", "https://www.youtube.com/watch?v=dQw4w9WgXcQ", "dQw4w9WgXcQ"},
		{"watch no www", "https://youtube.com/watch?v=dQw4w9WgXcQ", "dQw4w9WgXcQ"},
		{"short youtu.be", "https://youtu.be/dQw4w9WgXcQ", "dQw4w9WgXcQ"},
		{"embed", "https://www.youtube.com/embed/dQw4w9WgXcQ", "dQw4w9WgXcQ"},
		{"with playlist param", "https://www.youtube.com/watch?v=dQw4w9WgXcQ&list=PLabcdef", "dQw4w9WgXcQ"},
		{"short with timestamp", "https://youtu.be/dQw4w9WgXcQ?t=42", "dQw4w9WgXcQ"},
		{"params before v", "https://www.youtube.com/watch?feature=share&v=dQw4w9WgXcQ", "dQw4w9WgXcQ"},
		{"mobile", "https://m.youtube.com/watch?v=dQw4w9WgXcQ", "dQw4w9WgXcQ"},
		{"empty", "", ""},
		{"not youtube", "https://vimeo.com/123456", ""},
		{"garbage", "just some text", ""},
		{"id too short", "https://youtu.be/abc", ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := ExtractYouTubeID(tc.url); got != tc.want {
				t.Errorf("ExtractYouTubeID(%q) = %q, want %q", tc.url, got, tc.want)
			}
		})
	}
}
