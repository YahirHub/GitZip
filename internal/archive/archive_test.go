package archive

import (
	stdzip "archive/zip"
	"io"
	"os"
	"path/filepath"
	"testing"

	cryptozip "github.com/yeka/zip"

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

	reader, err := stdzip.OpenReader(archivePath)
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

func TestCollectPreservesDirectorySymlinkWithoutFollowingIt(t *testing.T) {
	root := t.TempDir()
	mustWrite(t, filepath.Join(root, "storage", "app", "public", "avatar.txt"), "target-data")
	if err := os.MkdirAll(filepath.Join(root, "public"), 0o755); err != nil {
		t.Fatalf("no se pudo crear public/: %v", err)
	}

	linkTarget := filepath.Join("..", "storage", "app", "public")
	linkPath := filepath.Join(root, "public", "storage")
	if err := os.Symlink(linkTarget, linkPath); err != nil {
		t.Skipf("el entorno no permite crear symlinks: %v", err)
	}

	archivePath := filepath.Join(root, filepath.Base(root)+".zip")
	entries, stats, err := Collect(root, archivePath)
	if err != nil {
		t.Fatalf("Collect devolvió error con symlink: %v", err)
	}

	bar := progress.New(io.Discard, stats.Bytes)
	if _, err := CreateZip(root, archivePath, entries, bar); err != nil {
		t.Fatalf("CreateZip devolvió error con symlink: %v", err)
	}

	reader, err := stdzip.OpenReader(archivePath)
	if err != nil {
		t.Fatalf("no se pudo abrir el ZIP con symlink: %v", err)
	}
	defer reader.Close()

	var linkEntry *stdzip.File
	for _, file := range reader.File {
		if file.Name == "public/storage" {
			linkEntry = file
			break
		}
	}
	if linkEntry == nil {
		t.Fatal("public/storage debería haberse preservado como entrada del ZIP")
	}
	if linkEntry.Mode()&os.ModeSymlink == 0 {
		t.Fatalf("public/storage debería conservar ModeSymlink, modo recibido: %s", linkEntry.Mode())
	}

	payloadReader, err := linkEntry.Open()
	if err != nil {
		t.Fatalf("no se pudo leer el payload del symlink: %v", err)
	}
	payload, err := io.ReadAll(payloadReader)
	payloadReader.Close()
	if err != nil {
		t.Fatalf("no se pudo leer el target del symlink: %v", err)
	}
	if string(payload) != linkTarget {
		t.Fatalf("target del symlink incorrecto: se obtuvo %q, se esperaba %q", payload, linkTarget)
	}
}

func TestCreateZipWithPasswordCanBeReadWithSamePassword(t *testing.T) {
	root := t.TempDir()
	mustWrite(t, filepath.Join(root, "README.md"), "archivo protegido")

	archivePath := filepath.Join(root, filepath.Base(root)+".zip")
	entries, stats, err := Collect(root, archivePath)
	if err != nil {
		t.Fatalf("Collect devolvió error: %v", err)
	}

	bar := progress.New(io.Discard, stats.Bytes)
	if _, err := CreateZipWithOptions(root, archivePath, entries, bar, CreateOptions{Password: "clave-prueba"}); err != nil {
		t.Fatalf("CreateZipWithOptions devolvió error: %v", err)
	}

	reader, err := cryptozip.OpenReader(archivePath)
	if err != nil {
		t.Fatalf("no se pudo abrir el ZIP cifrado: %v", err)
	}
	defer reader.Close()

	var found bool
	for _, file := range reader.File {
		if file.Name != "README.md" {
			continue
		}
		found = true
		if !file.IsEncrypted() {
			t.Fatal("README.md debería estar cifrado")
		}
		file.SetPassword("clave-prueba")
		contentReader, err := file.Open()
		if err != nil {
			t.Fatalf("no se pudo abrir README.md cifrado: %v", err)
		}
		content, err := io.ReadAll(contentReader)
		contentReader.Close()
		if err != nil {
			t.Fatalf("no se pudo leer README.md cifrado: %v", err)
		}
		if string(content) != "archivo protegido" {
			t.Fatalf("contenido descifrado inesperado: %q", content)
		}
	}
	if !found {
		t.Fatal("README.md debería existir dentro del ZIP cifrado")
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
