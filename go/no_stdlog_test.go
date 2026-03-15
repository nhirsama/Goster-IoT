package main

import (
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

func TestNoDirectStdLogInNonTestGoFiles(t *testing.T) {
	pattern := regexp.MustCompile(`\blog\.(Print|Printf|Println|Fatal|Fatalf|Panic|Panicf)\b`)
	roots := []string{"cli", "src"}

	for _, root := range roots {
		root := root
		if err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
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
				t.Errorf("forbidden std log call found in %s", path)
			}
			return nil
		}); err != nil {
			t.Fatalf("scan %s failed: %v", root, err)
		}
	}
}
