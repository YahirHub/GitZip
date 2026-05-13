package archive

import (
	"archive/zip"
	"os"
	"path/filepath"
	"testing"

	"gitzip/internal/progress"
)

func TestCollectRespectsNestedGitignoreAndCreatesZip(t *testing.T) {
	root := t.TempDir()

	mustWrite(t, filepath.Join(root, ".gitignore"), "node_modules/\n*.log\n")
	mustWrite(t, filepath.Join(root, "main.go"), "package main\n")
	mustWrite(t, filepath.Join(root, "debug.log"), "debug\n")
	mustWrite(t, filepath.Join(root, "node_modules", "pkg", "index.js"), "ignored\n")
	mustWrite(t, filepath.Join(root, "src", ".gitignore"), "secret.txt\n")
	mustWrite(t, filepath.Join(root, "src", "visible.txt"), "visible\n")
	mustWrite(t, filepath.Join(root, "src", "secret.txt"), "hidden\n")

	archivePath := filepath.Join(root, filepath.Base(root)+".zip")
	entries, stats, err := Collect(root, archivePath)
	if err != nil {
		t.Fatalf("Collect devolvió error: %v", err)
	}

	if stats.Files != 4 {
		t.Fatalf("se esperaban 4 archivos incluidos (.gitignore raíz, main.go, .gitignore anidado, visible.txt), llegaron %d", stats.Files)
	}

	bar := progress.New(os.Stdout, stats.Bytes)
	result, err := CreateZip(root, archivePath, entries, bar)
	bar.Finish(err == nil)
	if err != nil {
		t.Fatalf("CreateZip devolvió error: %v", err)
	}

	if result.Files != stats.Files {
		t.Fatalf("el ZIP reportó %d archivos, pero se esperaban %d", result.Files, stats.Files)
	}

	reader, err := zip.OpenReader(archivePath)
	if err != nil {
		t.Fatalf("no se pudo abrir el ZIP: %v", err)
	}
	defer reader.Close()

	found := map[string]bool{}
	for _, file := range reader.File {
		found[file.Name] = true
	}

	if !found["main.go"] {
		t.Fatal("main.go debería estar en el ZIP")
	}
	if !found["src/visible.txt"] {
		t.Fatal("src/visible.txt debería estar en el ZIP")
	}
	if found["debug.log"] {
		t.Fatal("debug.log no debería estar en el ZIP")
	}
	if found["node_modules/pkg/index.js"] {
		t.Fatal("node_modules/pkg/index.js no debería estar en el ZIP")
	}
	if found["src/secret.txt"] {
		t.Fatal("src/secret.txt no debería estar en el ZIP")
	}
}

func mustWrite(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("no se pudo crear carpeta para %s: %v", path, err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("no se pudo escribir %s: %v", path, err)
	}
}
