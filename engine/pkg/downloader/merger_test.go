package downloader

import (
	"strings"
	"testing"
)

func TestGetYtDlpVideoFormatArgs(t *testing.T) {
	tests := []struct {
		res        string
		hasFfmpeg  bool
		wantSubstr string
	}{
		{"1080p", true, "[height<=1080]"},
		{"720p", true, "[height<=720]"},
		{"480p", true, "[height<=480]"},
		{"360p", true, "[height<=360]"},
		{"best", true, "bestvideo[ext=mp4]"},
		{"1080p", false, "[height<=1080]"},
		{"best", false, "best[ext=mp4]/best"},
	}

	for _, tt := range tests {
		args := getYtDlpVideoFormatArgs(tt.res, tt.hasFfmpeg)
		found := false
		for _, arg := range args {
			if strings.Contains(arg, tt.wantSubstr) {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("getYtDlpVideoFormatArgs(%q, %v) = %v; missing %q", tt.res, tt.hasFfmpeg, args, tt.wantSubstr)
		}
	}
}
