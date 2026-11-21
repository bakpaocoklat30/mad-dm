package downloader

import (
	"os"
	"path/filepath"
	"strings"
)

type Category string

const (
	CategoryAudio     Category = "Audio"
	CategoryDocuments Category = "Documents"
	CategoryProgram   Category = "Program"
	CategoryArchive   Category = "Archive"
	CategoryVideo     Category = "Video"
	CategoryGeneral   Category = "General"
)

var extensionCategories = map[string]Category{
	// Audio
	".mp3":  CategoryAudio,
	".flac": CategoryAudio,
	".aac":  CategoryAudio,
	".m4a":  CategoryAudio,
	".wav":  CategoryAudio,
	".ogg":  CategoryAudio,
	".opus": CategoryAudio,
	".wma":  CategoryAudio,
	".alac": CategoryAudio,
	".aiff": CategoryAudio,
	".mid":  CategoryAudio,
	".midi": CategoryAudio,

	// Documents
	".pdf":     CategoryDocuments,
	".doc":     CategoryDocuments,
	".docx":    CategoryDocuments,
	".xls":     CategoryDocuments,
	".xlsx":    CategoryDocuments,
	".ppt":     CategoryDocuments,
	".pptx":    CategoryDocuments,
	".txt":     CategoryDocuments,
	".csv":     CategoryDocuments,
	".rtf":     CategoryDocuments,
	".odt":     CategoryDocuments,
	".ods":     CategoryDocuments,
	".odp":     CategoryDocuments,
	".epub":    CategoryDocuments,
	".pages":   CategoryDocuments,
	".numbers": CategoryDocuments,
	".keynote": CategoryDocuments,
	".md":      CategoryDocuments,
	".json":    CategoryDocuments,
	".xml":     CategoryDocuments,
	".log":     CategoryDocuments,
	".tex":     CategoryDocuments,

	// Program / Software
	".dmg":      CategoryProgram,
	".pkg":      CategoryProgram,
	".exe":      CategoryProgram,
	".msi":      CategoryProgram,
	".deb":      CategoryProgram,
	".rpm":      CategoryProgram,
	".appimage": CategoryProgram,
	".apk":      CategoryProgram,
	".ipa":      CategoryProgram,
	".sh":       CategoryProgram,
	".bat":      CategoryProgram,
	".cmd":      CategoryProgram,
	".com":      CategoryProgram,
	".bin":      CategoryProgram,
	".jar":      CategoryProgram,

	// Archive / Compressed
	".zip":     CategoryArchive,
	".rar":     CategoryArchive,
	".7z":      CategoryArchive,
	".tar":     CategoryArchive,
	".gz":      CategoryArchive,
	".bz2":     CategoryArchive,
	".xz":      CategoryArchive,
	".tgz":     CategoryArchive,
	".tbz2":    CategoryArchive,
	".iso":     CategoryArchive,
	".img":     CategoryArchive,
	".zst":     CategoryArchive,
	".cab":     CategoryArchive,
	".arj":     CategoryArchive,
	".zipx":    CategoryArchive,
	".tar.gz":  CategoryArchive,
	".tar.bz2": CategoryArchive,
	".tar.xz":  CategoryArchive,

	// Video
	".mp4":   CategoryVideo,
	".mkv":   CategoryVideo,
	".webm":  CategoryVideo,
	".avi":   CategoryVideo,
	".mov":   CategoryVideo,
	".flv":   CategoryVideo,
	".wmv":   CategoryVideo,
	".m4v":   CategoryVideo,
	".ts":    CategoryVideo,
	".mpg":   CategoryVideo,
	".mpeg":  CategoryVideo,
	".3gp":   CategoryVideo,
	".ogv":   CategoryVideo,
	".vob":   CategoryVideo,
	".m2ts":  CategoryVideo,
}

// ResolveCategory returns the media/file category based on filename and format
func ResolveCategory(filename, format string) Category {
	clean := filename
	// Strip query parameters or hashes if present in filename
	if idx := strings.IndexAny(clean, "?#"); idx != -1 {
		clean = clean[:idx]
	}
	cleanLower := strings.ToLower(clean)

	// Check multi-dot archive extensions first
	for multiExt, cat := range map[string]Category{
		".tar.gz":  CategoryArchive,
		".tar.bz2": CategoryArchive,
		".tar.xz":  CategoryArchive,
	} {
		if strings.HasSuffix(cleanLower, multiExt) {
			return cat
		}
	}

	ext := strings.ToLower(filepath.Ext(clean))
	if cat, found := extensionCategories[ext]; found {
		return cat
	}

	fmtLower := strings.ToLower(strings.TrimPrefix(format, "."))
	if fmtLower == "mp3" || fmtLower == "flac" || fmtLower == "aac" || fmtLower == "wav" {
		return CategoryAudio
	}
	if fmtLower == "mp4" || fmtLower == "mkv" || fmtLower == "webm" {
		return CategoryVideo
	}

	return CategoryGeneral
}

// CategorizedDir returns the categorized subfolder path inside baseDir and ensures it exists
func CategorizedDir(baseDir, filename, format string) string {
	if baseDir == "" {
		homeDir, _ := os.UserHomeDir()
		baseDir = filepath.Join(homeDir, "Downloads")
	}

	// If baseDir already ends with one of the category subfolders, don't nest again
	baseName := filepath.Base(baseDir)
	if strings.EqualFold(baseName, string(CategoryAudio)) ||
		strings.EqualFold(baseName, string(CategoryDocuments)) ||
		strings.EqualFold(baseName, string(CategoryProgram)) ||
		strings.EqualFold(baseName, string(CategoryArchive)) ||
		strings.EqualFold(baseName, string(CategoryVideo)) {
		_ = os.MkdirAll(baseDir, 0755)
		return baseDir
	}

	cat := ResolveCategory(filename, format)
	if cat == CategoryGeneral || cat == "" {
		_ = os.MkdirAll(baseDir, 0755)
		return baseDir
	}

	targetDir := filepath.Join(baseDir, string(cat))
	_ = os.MkdirAll(targetDir, 0755)
	return targetDir
}
