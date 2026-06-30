package tests

import (
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"testing"
)

func TestNoDirectStdLogInNonTestGoFiles(t *testing.T) {
	pattern := regexp.MustCompile(`\blog\.(Print|Printf|Println|Fatal|Fatalf|Panic|Panicf)\b`)
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("resolve test file path failed")
	}
	moduleRoot := filepath.Clean(filepath.Join(filepath.Dir(file), ".."))
	roots := []string{"cli", "src"}

	for _, root := range roots {
		root := root
		rootPath := filepath.Join(moduleRoot, root)
		if err := filepath.WalkDir(rootPath, func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if d.IsDir() {
				return nil
			}
			if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
				return nil
			}
			content, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			if pattern.Match(content) {
				relPath, relErr := filepath.Rel(moduleRoot, path)
				if relErr != nil {
					relPath = path
				}
				t.Errorf("forbidden std log call found in %s", relPath)
			}
			return nil
		}); err != nil {
			t.Fatalf("scan %s failed: %v", root, err)
		}
	}
}
