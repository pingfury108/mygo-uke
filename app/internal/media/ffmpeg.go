package media

import (
	"os"
	"path/filepath"

	"github.com/egoist/mygo"
)

// FFmpegPath finds ffmpeg in the app's resources first, then falls back to
// PATH. A packaged app puts the binary in resources/ next to the fonts.
func FFmpegPath() string {
	if value := os.Getenv("MYGO_UKE_FFMPEG"); value != "" {
		return value
	}
	if value := bundledTool("ffmpeg"); value != "" {
		return value
	}
	return "ffmpeg"
}

func bundledTool(name string) string {
	var candidates []string
	if dir, err := mygo.App.Path(mygo.PathResources); err == nil {
		candidates = append(candidates,
			filepath.Join(dir, name),
			filepath.Join(dir, "bin", name),
		)
	}
	if exe, err := os.Executable(); err == nil {
		candidates = append(candidates, filepath.Join(filepath.Dir(exe), name))
	}
	for _, path := range candidates {
		info, err := os.Stat(path)
		if err != nil || info.IsDir() {
			continue
		}
		if info.Mode().Perm()&0o111 == 0 {
			_ = os.Chmod(path, 0o755)
		}
		return path
	}
	return ""
}
