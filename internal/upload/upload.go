package upload

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const maxResponseBytes = 128 * 1024

// Result describes a successful temporary upload.
type Result struct {
	Provider string
	URL      string
}

// Attempt captures a provider failure before the fallback chain continues.
type Attempt struct {
	Provider string
	Err      error
}

// Provider uploads a local archive and returns a download URL.
type Provider interface {
	Name() string
	Upload(ctx context.Context, client *http.Client, archivePath string) (string, error)
}

// Upload tries documented temporary-upload providers in order until one works.
func Upload(ctx context.Context, archivePath string) (Result, []Attempt, error) {
	client := &http.Client{Timeout: 15 * time.Minute}
	return uploadWithProviders(ctx, client, archivePath, defaultProviders())
}

func uploadWithProviders(ctx context.Context, client *http.Client, archivePath string, providers []Provider) (Result, []Attempt, error) {
	if strings.TrimSpace(archivePath) == "" {
		return Result{}, nil, errors.New("la ruta del ZIP no puede estar vacía")
	}
	if len(providers) == 0 {
		return Result{}, nil, errors.New("no hay proveedores de subida configurados")
	}
	if client == nil {
		client = &http.Client{Timeout: 15 * time.Minute}
	}

	archiveInfo, err := os.Stat(archivePath)
	if err != nil {
		return Result{}, nil, fmt.Errorf("no se pudo verificar el ZIP a subir: %w", err)
	}
	if !archiveInfo.Mode().IsRegular() {
		return Result{}, nil, fmt.Errorf("la ruta a subir no es un archivo ZIP regular: %s", archivePath)
	}

	attempts := make([]Attempt, 0, len(providers))
	for _, provider := range providers {
		if err := ctx.Err(); err != nil {
			return Result{}, attempts, err
		}

		rawURL, err := provider.Upload(ctx, client, archivePath)
		if err != nil {
			attempts = append(attempts, Attempt{Provider: provider.Name(), Err: err})
			continue
		}

		downloadURL, err := normalizeHTTPURL(rawURL)
		if err != nil {
			attempts = append(attempts, Attempt{Provider: provider.Name(), Err: err})
			continue
		}

		return Result{Provider: provider.Name(), URL: downloadURL}, attempts, nil
	}

	return Result{}, attempts, summarizeFailures(attempts)
}

func defaultProviders() []Provider {
	return []Provider{
		&plainMultipartProvider{
			name:      "temp.sh",
			endpoint:  "https://temp.sh/upload",
			fileField: "file",
		},
		&plainMultipartProvider{
			name:      "Litterbox",
			endpoint:  "https://litterbox.catbox.moe/resources/internals/api.php",
			fileField: "fileToUpload",
			fields: map[string]string{
				"reqtype": "fileupload",
				"time":    "72h",
			},
		},
		&fileIOProvider{endpoint: "https://file.io/?expires=1w"},
		&tmpFilesProvider{endpoint: "https://tmpfiles.org/api/v1/upload"},
		&plainMultipartProvider{
			name:      "0x0.st",
			endpoint:  "https://0x0.st",
			fileField: "file",
		},
	}
}

type plainMultipartProvider struct {
	name      string
	endpoint  string
	fileField string
	fields    map[string]string
}

func (p *plainMultipartProvider) Name() string {
	return p.name
}

func (p *plainMultipartProvider) Upload(ctx context.Context, client *http.Client, archivePath string) (string, error) {
	payload, err := multipartUpload(ctx, client, p.endpoint, p.fileField, archivePath, p.fields)
	if err != nil {
		return "", err
	}
	return parsePlainURL(payload)
}

type fileIOProvider struct {
	endpoint string
}

func (p *fileIOProvider) Name() string {
	return "file.io"
}

func (p *fileIOProvider) Upload(ctx context.Context, client *http.Client, archivePath string) (string, error) {
	payload, err := multipartUpload(ctx, client, p.endpoint, "file", archivePath, nil)
	if err != nil {
		return "", err
	}

	var response struct {
		Success bool   `json:"success"`
		Link    string `json:"link"`
		Message string `json:"message"`
		Error   any    `json:"error"`
	}
	if err := json.Unmarshal(payload, &response); err != nil {
		return "", fmt.Errorf("file.io devolvió JSON inválido: %w", err)
	}
	if !response.Success || strings.TrimSpace(response.Link) == "" {
		message := strings.TrimSpace(response.Message)
		if message == "" && response.Error != nil {
			message = fmt.Sprint(response.Error)
		}
		if message == "" {
			message = "respuesta sin enlace"
		}
		return "", fmt.Errorf("file.io rechazó la subida: %s", message)
	}
	return response.Link, nil
}

type tmpFilesProvider struct {
	endpoint string
}

func (p *tmpFilesProvider) Name() string {
	return "tmpfiles.org"
}

func (p *tmpFilesProvider) Upload(ctx context.Context, client *http.Client, archivePath string) (string, error) {
	payload, err := multipartUpload(ctx, client, p.endpoint, "file", archivePath, map[string]string{"expire": "172800"})
	if err != nil {
		return "", err
	}

	var response struct {
		Status string `json:"status"`
		Data   struct {
			URL string `json:"url"`
		} `json:"data"`
	}
	if err := json.Unmarshal(payload, &response); err != nil {
		return "", fmt.Errorf("tmpfiles.org devolvió JSON inválido: %w", err)
	}
	if response.Status != "success" || strings.TrimSpace(response.Data.URL) == "" {
		return "", errors.New("tmpfiles.org no devolvió un enlace de descarga")
	}
	return response.Data.URL, nil
}

func multipartUpload(ctx context.Context, client *http.Client, endpoint, fileField, archivePath string, fields map[string]string) ([]byte, error) {
	if strings.TrimSpace(endpoint) == "" {
		return nil, errors.New("el endpoint de subida no puede estar vacío")
	}
	if strings.TrimSpace(fileField) == "" {
		return nil, errors.New("el campo multipart del archivo no puede estar vacío")
	}

	file, err := os.Open(archivePath)
	if err != nil {
		return nil, fmt.Errorf("no se pudo abrir el ZIP para subirlo: %w", err)
	}

	pipeReader, pipeWriter := io.Pipe()
	form := multipart.NewWriter(pipeWriter)
	writeDone := make(chan error, 1)

	go func() {
		defer file.Close()

		for key, value := range fields {
			if err := form.WriteField(key, value); err != nil {
				_ = pipeWriter.CloseWithError(err)
				writeDone <- err
				return
			}
		}

		part, err := form.CreateFormFile(fileField, filepath.Base(archivePath))
		if err != nil {
			_ = pipeWriter.CloseWithError(err)
			writeDone <- err
			return
		}
		if _, err := io.Copy(part, file); err != nil {
			_ = pipeWriter.CloseWithError(err)
			writeDone <- err
			return
		}
		if err := form.Close(); err != nil {
			_ = pipeWriter.CloseWithError(err)
			writeDone <- err
			return
		}
		writeDone <- pipeWriter.Close()
	}()

	request, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, pipeReader)
	if err != nil {
		_ = pipeReader.Close()
		return nil, fmt.Errorf("no se pudo crear la solicitud HTTP: %w", err)
	}
	request.Header.Set("Content-Type", form.FormDataContentType())
	request.Header.Set("User-Agent", "gitzip-upload/1.0")

	response, err := client.Do(request)
	if err != nil {
		_ = pipeReader.CloseWithError(err)
		select {
		case writeErr := <-writeDone:
			if writeErr != nil {
				return nil, fmt.Errorf("falló la subida HTTP: %w; escritura multipart: %v", err, writeErr)
			}
		default:
		}
		return nil, fmt.Errorf("falló la subida HTTP: %w", err)
	}
	defer response.Body.Close()

	payload, readErr := io.ReadAll(io.LimitReader(response.Body, maxResponseBytes))
	writeErr := <-writeDone
	if writeErr != nil {
		return nil, fmt.Errorf("falló la escritura multipart: %w", writeErr)
	}
	if readErr != nil {
		return nil, fmt.Errorf("no se pudo leer la respuesta de subida: %w", readErr)
	}
	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		body := strings.TrimSpace(string(payload))
		if body == "" {
			body = response.Status
		}
		return nil, fmt.Errorf("el servidor respondió %s: %s", response.Status, body)
	}
	return payload, nil
}

func parsePlainURL(payload []byte) (string, error) {
	text := strings.TrimSpace(string(payload))
	if text == "" {
		return "", errors.New("el proveedor respondió vacío")
	}
	lines := strings.Fields(text)
	if len(lines) == 0 {
		return "", errors.New("el proveedor no devolvió un enlace")
	}
	return lines[0], nil
}

func normalizeHTTPURL(raw string) (string, error) {
	candidate := strings.TrimSpace(raw)
	parsed, err := url.Parse(candidate)
	if err != nil {
		return "", fmt.Errorf("URL de descarga inválida: %w", err)
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return "", fmt.Errorf("URL de descarga sin esquema HTTP válido: %q", candidate)
	}
	if strings.TrimSpace(parsed.Host) == "" {
		return "", fmt.Errorf("URL de descarga sin host: %q", candidate)
	}
	return parsed.String(), nil
}

func summarizeFailures(attempts []Attempt) error {
	if len(attempts) == 0 {
		return errors.New("ningún proveedor de subida fue ejecutado")
	}
	parts := make([]string, 0, len(attempts))
	for _, attempt := range attempts {
		parts = append(parts, fmt.Sprintf("%s: %v", attempt.Provider, attempt.Err))
	}
	return fmt.Errorf("todos los proveedores temporales fallaron: %s", strings.Join(parts, " | "))
}
