package app

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"gitzip/internal/archive"
	"gitzip/internal/progress"
	"gitzip/internal/upload"
)

const Version = "0.2.0"

// Run executes the gitzip CLI using the current working directory as project root.
func Run(stdout, stderr io.Writer, args []string) error {
	mode, err := parseMode(args)
	if err != nil {
		return fail(stderr, "comando inválido", err)
	}
	if mode == modeHelp {
		writeUsage(stdout)
		return nil
	}

	cwd, err := os.Getwd()
	if err != nil {
		return fail(stderr, "no se pudo obtener la carpeta actual", err)
	}

	root, err := filepath.Abs(cwd)
	if err != nil {
		return fail(stderr, "no se pudo resolver la ruta absoluta", err)
	}

	folderName := normalizedFolderName(root)
	archivePath := filepath.Join(root, folderName+".zip")

	fmt.Fprintf(stdout, "gitzip v%s\n", Version)
	fmt.Fprintf(stdout, "Proyecto: %s\n", folderName)
	fmt.Fprintf(stdout, "Salida:   %s\n", archivePath)
	if mode == modeUpload {
		fmt.Fprintln(stdout, "Modo:     upload")
	}
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
	writeSkippedSummary(stdout, stats.Skipped)

	password := ""
	if mode == modeUpload {
		password, err = randomPassword()
		if err != nil {
			return fail(stderr, "no se pudo generar la contraseña aleatoria", err)
		}
	}

	bar := progress.New(stdout, stats.Bytes)
	result, err := archive.CreateZipWithOptions(root, archivePath, entries, bar, archive.CreateOptions{Password: password})
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

	if mode == modeUpload {
		if err := uploadArchive(stdout, archivePath, password); err != nil {
			return fail(stderr, "falló la subida temporal; el ZIP cifrado quedó creado localmente", err)
		}
	}

	return nil
}

type runMode uint8

const (
	modeZip runMode = iota
	modeUpload
	modeHelp
)

func parseMode(args []string) (runMode, error) {
	if len(args) == 0 {
		return modeZip, nil
	}
	if len(args) > 1 {
		return modeZip, fmt.Errorf("se esperaba cero argumentos o un único comando, llegaron %d", len(args))
	}

	switch strings.TrimSpace(args[0]) {
	case "upload":
		return modeUpload, nil
	case "help", "--help", "-h":
		return modeHelp, nil
	default:
		return modeZip, fmt.Errorf("%q no es un comando soportado", args[0])
	}
}

func writeUsage(stdout io.Writer) {
	fmt.Fprintf(stdout, "gitzip v%s\n\n", Version)
	fmt.Fprintln(stdout, "Uso:")
	fmt.Fprintln(stdout, "  gitzip          Comprime el proyecto actual")
	fmt.Fprintln(stdout, "  gitzip upload   Comprime con contraseña aleatoria y sube a un host temporal")
	fmt.Fprintln(stdout, "  gitzip help     Muestra esta ayuda")
}

func normalizedFolderName(root string) string {
	folderName := strings.TrimSpace(filepath.Base(root))
	if folderName == "" || folderName == "." || folderName == string(filepath.Separator) {
		return "proyecto"
	}
	return folderName
}

func randomPassword() (string, error) {
	buffer := make([]byte, 24)
	if _, err := rand.Read(buffer); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(buffer), nil
}

func uploadArchive(stdout io.Writer, archivePath, password string) error {
	fmt.Fprintln(stdout)
	fmt.Fprintln(stdout, "Subiendo ZIP cifrado a proveedores temporales...")

	result, attempts, err := upload.Upload(context.Background(), archivePath)
	if len(attempts) > 0 {
		fmt.Fprintf(stdout, "Fallback activado: %d proveedor(es) fallaron antes del éxito o del error final.\n", len(attempts))
		for _, attempt := range attempts {
			fmt.Fprintf(stdout, "- %s: %v\n", attempt.Provider, attempt.Err)
		}
	}
	if err != nil {
		return err
	}

	archiveName := filepath.Base(archivePath)
	fmt.Fprintf(stdout, "Subida temporal completada con: %s\n", result.Provider)
	fmt.Fprintf(stdout, "Enlace: %s\n", result.URL)
	fmt.Fprintf(stdout, "Contraseña ZIP: %s\n", password)
	fmt.Fprintln(stdout, "Comando wget:")
	fmt.Fprintf(stdout, "wget -O %s %s\n", shellQuote(archiveName), shellQuote(result.URL))
	fmt.Fprintln(stdout, "Comando unzip:")
	fmt.Fprintf(stdout, "unzip -P %s %s\n", shellQuote(password), shellQuote(archiveName))
	return nil
}

func writeSkippedSummary(stdout io.Writer, skipped []archive.SkippedEntry) {
	if len(skipped) == 0 {
		return
	}

	fmt.Fprintf(stdout, "Omitidos por tipo especial no preservable: %d\n", len(skipped))
	limit := len(skipped)
	if limit > 5 {
		limit = 5
	}
	for _, item := range skipped[:limit] {
		fmt.Fprintf(stdout, "- %s (%s)\n", item.RelativePath, item.Reason)
	}
	if len(skipped) > limit {
		fmt.Fprintf(stdout, "- ... y %d más\n", len(skipped)-limit)
	}
}

func shellQuote(value string) string {
	return "'" + strings.ReplaceAll(value, "'", "'\\''") + "'"
}

func fail(stderr io.Writer, message string, err error) error {
	wrapped := fmt.Errorf("%s: %w", message, err)
	fmt.Fprintf(stderr, "Error: %v\n", wrapped)
	return wrapped
}
