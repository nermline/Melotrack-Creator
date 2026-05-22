package media

import "testing"

// GetVideoMetadata має повертати нулі для неіснуючого файлу (або коли ffprobe
// недоступний) — це сигнал «пропустити валідацію».
func TestGetVideoMetadata_MissingFile(t *testing.T) {
	w, h, d := GetVideoMetadata("/definitely/not/a/real/file.mp4")
	if w != 0 || h != 0 || d != 0 {
		t.Errorf("expected 0,0,0 for missing file, got w=%d h=%d d=%f", w, h, d)
	}
}
