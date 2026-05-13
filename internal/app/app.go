package app

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"gitzip/internal/archive"
	"gitzip/internal/progress"
)

const Version = "0.1.0"

// Run executes the gitzip CLI using the current working directory as project root.
func Run(stdout, stderr io.Writer) error {
	cwd, err := os.Getwd()
	if err != nil {
		return fail(stderr, "no se pudo obtener la carpeta actual", err)
	}

	root, err := filepath.Abs(cwd)
	if err != nil {
		return fail(stderr, "no se pudo resolver la ruta absoluta", err)
	}

	folderName := filepath.Base(root)
	folderName = strings.TrimSpace(folderName)
	if folderName == "" || folderName == "." || folderName == string(filepath.Separator) {
		folderName = "proyecto"
	}

	archivePath := filepath.Join(root, folderName+".zip")

	fmt.Fprintf(stdout, "gitzip v%s\n", Version)
	fmt.Fprintf(stdout, "Proyecto: %s\n", folderName)
	fmt.Fprintf(stdout, "Salida:   %s\n", archivePath)
	fmt.Fprintln(stdout, "Escaneando archivos respetando .gitignore...")

	startedAt := time.Now()
	entries, stats, err := archive.Collect(root, archivePath)
	if err != nil {
		return fail(stderr, "falló el escaneo del proyecto", err)
	}

	fmt.Fprintf(stdout, "Incluidos: %d archivos, %d carpetas, %s a procesar\n",
		stats.Files,
		stats.Directories,
		progress.FormatBytes(stats.Bytes),
	)

	bar := progress.New(stdout, stats.Bytes)
	result, err := archive.CreateZip(root, archivePath, entries, bar)
	if err != nil {
		bar.Finish(false)
		return fail(stderr, "falló la creación del ZIP", err)
	}
	bar.Finish(true)

	elapsed := time.Since(startedAt).Round(time.Millisecond)
	fmt.Fprintln(stdout)
	fmt.Fprintf(stdout, "ZIP creado correctamente: %s\n", result.ArchivePath)
	fmt.Fprintf(stdout, "Contenido: %d archivos, %d carpetas\n", result.Files, result.Directories)
	fmt.Fprintf(stdout, "Tamaño lógico procesado: %s\n", progress.FormatBytes(result.Bytes))
	fmt.Fprintf(stdout, "Tiempo total: %s\n", elapsed)

	return nil
}

func fail(stderr io.Writer, message string, err error) error {
	wrapped := fmt.Errorf("%s: %w", message, err)
	fmt.Fprintf(stderr, "Error: %v\n", wrapped)
	return wrapped
}
