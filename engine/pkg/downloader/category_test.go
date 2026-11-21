package downloader

import (
	"path/filepath"
	"testing"
)

func TestResolveCategory(t *testing.T) {
	tests := []struct {
		filename string
		format   string
		expected Category
	}{
		// Audio
		{"song.mp3", "", CategoryAudio},
		{"audio.flac", "", CategoryAudio},
		{"podcast.aac", "", CategoryAudio},
		{"sound.wav", "", CategoryAudio},
		{"speech.ogg", "", CategoryAudio},
		{"track.opus", "", CategoryAudio},
		{"voice.m4a", "", CategoryAudio},
		{"music.wma", "", CategoryAudio},

		// Documents
		{"report.pdf", "", CategoryDocuments},
		{"doc.docx", "", CategoryDocuments},
		{"legacy.doc", "", CategoryDocuments},
		{"sheet.xlsx", "", CategoryDocuments},
		{"table.xls", "", CategoryDocuments},
		{"slides.pptx", "", CategoryDocuments},
		{"notes.txt", "", CategoryDocuments},
		{"data.csv", "", CategoryDocuments},
		{"book.epub", "", CategoryDocuments},

		// Program / Software
		{"installer.dmg", "", CategoryProgram},
		{"setup.exe", "", CategoryProgram},
		{"bundle.pkg", "", CategoryProgram},
		{"package.deb", "", CategoryProgram},
		{"app.appimage", "", CategoryProgram},
		{"script.sh", "", CategoryProgram},
		{"app.apk", "", CategoryProgram},

		// Archive / Compressed
		{"archive.zip", "", CategoryArchive},
		{"compressed.rar", "", CategoryArchive},
		{"backup.7z", "", CategoryArchive},
		{"tarball.tar.gz", "", CategoryArchive},
		{"tarball.tar.bz2", "", CategoryArchive},
		{"source.tar.xz", "", CategoryArchive},
		{"image.iso", "", CategoryArchive},

		// Video
		{"video.mp4", "", CategoryVideo},
		{"movie.mkv", "", CategoryVideo},
		{"clip.webm", "", CategoryVideo},
		{"film.avi", "", CategoryVideo},
		{"recording.mov", "", CategoryVideo},

		// Format fallback
		{"", "mp3", CategoryAudio},
		{"", "flac", CategoryAudio},
		{"", "mp4", CategoryVideo},

		// Query params and fragments
		{"doc.pdf?auth=123&exp=456", "", CategoryDocuments},
		{"movie.mp4#t=10", "", CategoryVideo},
		{"archive.zip?download=true", "", CategoryArchive},
	}

	for _, tt := range tests {
		actual := ResolveCategory(tt.filename, tt.format)
		if actual != tt.expected {
			t.Errorf("ResolveCategory(%q, %q) = %v; want %v", tt.filename, tt.format, actual, tt.expected)
		}
	}
}

func TestCategorizedDir(t *testing.T) {
	base := "/tmp/Downloads"

	tests := []struct {
		baseDir  string
		filename string
		format   string
		expected string
	}{
		{base, "test.pdf", "", filepath.Join(base, "Documents")},
		{base, "music.mp3", "", filepath.Join(base, "Audio")},
		{base, "setup.dmg", "", filepath.Join(base, "Program")},
		{base, "archive.zip", "", filepath.Join(base, "Archive")},
		{base, "movie.mp4", "", filepath.Join(base, "Video")},
		// No double-nesting if already inside category
		{filepath.Join(base, "Audio"), "music.mp3", "", filepath.Join(base, "Audio")},
		{filepath.Join(base, "Documents"), "doc.pdf", "", filepath.Join(base, "Documents")},
	}

	for _, tt := range tests {
		actual := CategorizedDir(tt.baseDir, tt.filename, tt.format)
		if actual != tt.expected {
			t.Errorf("CategorizedDir(%q, %q, %q) = %q; want %q", tt.baseDir, tt.filename, tt.format, actual, tt.expected)
		}
	}
}
