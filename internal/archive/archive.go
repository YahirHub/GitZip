package archive

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	gitignore "github.com/git-pkgs/gitignore"
	zip "github.com/yeka/zip"

	"gitzip/internal/progress"
)

// EntryKind describes the kind of filesystem item to include in the archive.
type EntryKind uint8

const (
	EntryFile EntryKind = iota
	EntryDirectory
	EntrySymlink
)

// Entry is a normalized archive item discovered during project scanning.
type Entry struct {
	AbsolutePath string
	RelativePath string
	Kind         EntryKind
	Info         fs.FileInfo
	LinkTarget   string
	LogicalSize  int64
}

// SkippedEntry describes an item intentionally omitted because it cannot be
// preserved safely as a regular ZIP entry.
type SkippedEntry struct {
	RelativePath string
	Reason       string
}

// Stats describes the filtered project contents before compression.
type Stats struct {
	Files       int
	Directories int
	Bytes       int64
	Skipped     []SkippedEntry
}

// Result describes the ZIP contents that were written successfully.
type Result struct {
	ArchivePath string
	Files       int
	Directories int
	Bytes       int64
}

// CreateOptions controls optional ZIP generation behavior.
type CreateOptions struct {
	// Password enables Standard ZIP Encryption for regular files and symbolic
	// links. This is intentionally compatible with `unzip -P`, so it should be
	// treated as transport obfuscation rather than modern strong encryption.
	Password string
}

// Collect walks root respecting all nested .gitignore files and collects archive entries.
func Collect(root, archivePath string) ([]Entry, Stats, error) {
	root = filepath.Clean(root)
	archivePath = filepath.Clean(archivePath)

	var (
		entries []Entry
		stats   Stats
	)

	err := gitignore.Walk(root, func(relPath string, d fs.DirEntry) error {
		if strings.TrimSpace(relPath) == "" {
			return nil
		}

		absolutePath := filepath.Join(root, relPath)
		if samePath(absolutePath, archivePath) {
			return nil
		}

		// Use Lstat explicitly instead of DirEntry.Info. This keeps the logic
		// centered on the link/reparse point itself rather than the destination
		// and avoids treating broken symlinks as failed regular files.
		info, err := os.Lstat(absolutePath)
		if err != nil {
			return fmt.Errorf("no se pudo leer información de %s: %w", relPath, err)
		}

		entry := Entry{
			AbsolutePath: absolutePath,
			RelativePath: filepath.ToSlash(relPath),
			Info:         info,
		}

		mode := info.Mode()
		switch {
		case mode&os.ModeSymlink != 0:
			target, err := os.Readlink(absolutePath)
			if err != nil {
				return fmt.Errorf("no se pudo leer el enlace simbólico %s: %w", relPath, err)
			}
			entry.Kind = EntrySymlink
			entry.LinkTarget = target
			entry.LogicalSize = int64(len([]byte(target)))
			stats.Files++
			stats.Bytes += entry.LogicalSize
		case mode.IsDir():
			entry.Kind = EntryDirectory
			entry.LogicalSize = 0
			stats.Directories++
		case mode.IsRegular():
			entry.Kind = EntryFile
			entry.LogicalSize = info.Size()
			stats.Files++
			stats.Bytes += info.Size()
		default:
			// Go 1.23 on Windows reports some non-symlink reparse points,
			// including junctions, as ModeIrregular. Trying to os.Open them as
			// normal files breaks ZIP creation. The same guard also prevents
			// hangs or surprises with sockets, FIFOs and device files on Unix.
			stats.Skipped = append(stats.Skipped, SkippedEntry{
				RelativePath: filepath.ToSlash(relPath),
				Reason:       unsupportedModeReason(mode),
			})
			return nil
		}

		entries = append(entries, entry)
		return nil
	})
	if err != nil {
		return nil, Stats{}, err
	}

	return entries, stats, nil
}

// CreateZip writes all collected entries into archivePath without a password.
func CreateZip(root, archivePath string, entries []Entry, bar *progress.Bar) (Result, error) {
	return CreateZipWithOptions(root, archivePath, entries, bar, CreateOptions{})
}

// CreateZipWithOptions writes all collected entries into archivePath.
func CreateZipWithOptions(root, archivePath string, entries []Entry, bar *progress.Bar, options CreateOptions) (Result, error) {
	if bar == nil {
		return Result{}, errors.New("la barra de progreso no puede ser nil")
	}

	file, err := os.Create(archivePath)
	if err != nil {
		return Result{}, fmt.Errorf("no se pudo crear el archivo %s: %w", archivePath, err)
	}

	cleanup := true
	defer func() {
		_ = file.Close()
		if cleanup {
			_ = os.Remove(archivePath)
		}
	}()

	zipWriter := zip.NewWriter(file)
	result := Result{ArchivePath: archivePath}

	for _, entry := range entries {
		switch entry.Kind {
		case EntryDirectory:
			if err := addDirectory(zipWriter, entry); err != nil {
				_ = zipWriter.Close()
				return Result{}, err
			}
			result.Directories++
		case EntrySymlink:
			written, err := addSymlink(zipWriter, entry, bar, options.Password)
			if err != nil {
				_ = zipWriter.Close()
				return Result{}, err
			}
			result.Files++
			result.Bytes += written
		case EntryFile:
			written, err := addFile(zipWriter, entry, bar, options.Password)
			if err != nil {
				_ = zipWriter.Close()
				return Result{}, err
			}
			result.Files++
			result.Bytes += written
		default:
			_ = zipWriter.Close()
			return Result{}, fmt.Errorf("tipo de entrada no soportado: %d", entry.Kind)
		}
	}

	if err := zipWriter.Close(); err != nil {
		return Result{}, fmt.Errorf("no se pudo finalizar el ZIP: %w", err)
	}

	if err := file.Sync(); err != nil {
		return Result{}, fmt.Errorf("no se pudo sincronizar el ZIP en disco: %w", err)
	}

	cleanup = false
	return result, nil
}

func addDirectory(writer *zip.Writer, entry Entry) error {
	header, err := zip.FileInfoHeader(entry.Info)
	if err != nil {
		return fmt.Errorf("no se pudo preparar la carpeta %s: %w", entry.RelativePath, err)
	}

	header.Name = ensureDirSuffix(entry.RelativePath)
	header.Method = zip.Store

	if _, err := writer.CreateHeader(header); err != nil {
		return fmt.Errorf("no se pudo agregar la carpeta %s: %w", entry.RelativePath, err)
	}

	return nil
}

func addSymlink(writer *zip.Writer, entry Entry, bar *progress.Bar, password string) (int64, error) {
	header, err := zip.FileInfoHeader(entry.Info)
	if err != nil {
		return 0, fmt.Errorf("no se pudo preparar el enlace %s: %w", entry.RelativePath, err)
	}

	header.Name = entry.RelativePath
	header.Method = zip.Store
	header.SetMode(os.ModeSymlink | 0o777)
	applyPassword(header, password)

	archiveWriter, err := writer.CreateHeader(header)
	if err != nil {
		return 0, fmt.Errorf("no se pudo agregar el enlace %s: %w", entry.RelativePath, err)
	}

	payload := []byte(entry.LinkTarget)
	n, err := archiveWriter.Write(payload)
	if err != nil {
		return int64(n), fmt.Errorf("no se pudo escribir el enlace %s: %w", entry.RelativePath, err)
	}

	bar.Add(int64(n))
	return int64(n), nil
}

func addFile(writer *zip.Writer, entry Entry, bar *progress.Bar, password string) (int64, error) {
	header, err := zip.FileInfoHeader(entry.Info)
	if err != nil {
		return 0, fmt.Errorf("no se pudo preparar el archivo %s: %w", entry.RelativePath, err)
	}

	header.Name = entry.RelativePath
	header.Method = zip.Deflate
	applyPassword(header, password)

	archiveWriter, err := writer.CreateHeader(header)
	if err != nil {
		return 0, fmt.Errorf("no se pudo crear la entrada ZIP %s: %w", entry.RelativePath, err)
	}

	source, err := os.Open(entry.AbsolutePath)
	if err != nil {
		return 0, fmt.Errorf("no se pudo abrir %s: %w", entry.RelativePath, err)
	}
	defer source.Close()

	reader := progress.NewCountingReader(source, bar)
	written, err := io.Copy(archiveWriter, reader)
	if err != nil {
		return written, fmt.Errorf("no se pudo comprimir %s: %w", entry.RelativePath, err)
	}

	return written, nil
}

func applyPassword(header *zip.FileHeader, password string) {
	if password == "" {
		return
	}
	header.SetPassword(password)
	header.SetEncryptionMethod(zip.StandardEncryption)
}

func ensureDirSuffix(path string) string {
	if strings.HasSuffix(path, "/") {
		return path
	}
	return path + "/"
}

func samePath(left, right string) bool {
	leftAbs, leftErr := filepath.Abs(left)
	rightAbs, rightErr := filepath.Abs(right)
	if leftErr != nil || rightErr != nil {
		return filepath.Clean(left) == filepath.Clean(right)
	}

	return filepath.Clean(leftAbs) == filepath.Clean(rightAbs)
}

func unsupportedModeReason(mode fs.FileMode) string {
	switch {
	case mode&os.ModeIrregular != 0:
		return "entrada irregular o reparse point no preservable como archivo ZIP"
	case mode&os.ModeSocket != 0:
		return "socket omitido"
	case mode&os.ModeNamedPipe != 0:
		return "pipe/FIFO omitido"
	case mode&os.ModeDevice != 0:
		return "dispositivo omitido"
	default:
		return fmt.Sprintf("tipo especial omitido (%s)", mode.String())
	}
}
