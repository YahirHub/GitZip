package upload

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

type fakeProvider struct {
	name string
	url  string
	err  error
}

func (p fakeProvider) Name() string { return p.name }

func (p fakeProvider) Upload(_ context.Context, _ *http.Client, _ string) (string, error) {
	return p.url, p.err
}

func TestUploadWithProvidersFallsBackUntilSuccess(t *testing.T) {
	archivePath := mustArchive(t)
	result, attempts, err := uploadWithProviders(context.Background(), &http.Client{}, archivePath, []Provider{
		fakeProvider{name: "fallido", err: errors.New("sin servicio")},
		fakeProvider{name: "exitoso", url: "https://files.example/proyecto.zip"},
	})
	if err != nil {
		t.Fatalf("uploadWithProviders devolvió error: %v", err)
	}
	if result.Provider != "exitoso" {
		t.Fatalf("proveedor inesperado: %s", result.Provider)
	}
	if result.URL != "https://files.example/proyecto.zip" {
		t.Fatalf("URL inesperada: %s", result.URL)
	}
	if len(attempts) != 1 || attempts[0].Provider != "fallido" {
		t.Fatalf("intentos de fallback inesperados: %#v", attempts)
	}
}

func TestUploadWithProvidersRejectsMissingArchiveBeforeFallback(t *testing.T) {
	_, attempts, err := uploadWithProviders(context.Background(), &http.Client{}, filepath.Join(t.TempDir(), "no-existe.zip"), []Provider{
		fakeProvider{name: "no-debería-ejecutarse", url: "https://files.example/never.zip"},
	})
	if err == nil {
		t.Fatal("se esperaba error para ZIP inexistente")
	}
	if len(attempts) != 0 {
		t.Fatalf("no se debieron ejecutar proveedores; intentos: %#v", attempts)
	}
}

func TestPlainMultipartProviderUploadsFileAndParsesURL(t *testing.T) {
	archivePath := mustArchive(t)
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if err := request.ParseMultipartForm(4 << 20); err != nil {
			http.Error(writer, err.Error(), http.StatusBadRequest)
			return
		}
		file, _, err := request.FormFile("file")
		if err != nil {
			http.Error(writer, err.Error(), http.StatusBadRequest)
			return
		}
		defer file.Close()
		payload, err := io.ReadAll(file)
		if err != nil {
			http.Error(writer, err.Error(), http.StatusInternalServerError)
			return
		}
		if string(payload) != "zip-data" {
			http.Error(writer, "payload inesperado", http.StatusBadRequest)
			return
		}
		fmt.Fprintln(writer, "https://files.example/subida.zip")
	}))
	defer server.Close()

	provider := &plainMultipartProvider{name: "mock", endpoint: server.URL, fileField: "file"}
	result, err := provider.Upload(context.Background(), server.Client(), archivePath)
	if err != nil {
		t.Fatalf("provider.Upload devolvió error: %v", err)
	}
	if result != "https://files.example/subida.zip" {
		t.Fatalf("URL inesperada: %s", result)
	}
}

func TestFileIOProviderParsesJSONResponse(t *testing.T) {
	archivePath := mustArchive(t)
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if !strings.HasPrefix(request.Header.Get("Content-Type"), "multipart/form-data") {
			http.Error(writer, "content type inválido", http.StatusBadRequest)
			return
		}
		writer.Header().Set("Content-Type", "application/json")
		fmt.Fprint(writer, `{"success":true,"link":"https://file.io/mock"}`)
	}))
	defer server.Close()

	provider := &fileIOProvider{endpoint: server.URL}
	result, err := provider.Upload(context.Background(), server.Client(), archivePath)
	if err != nil {
		t.Fatalf("fileIOProvider devolvió error: %v", err)
	}
	if result != "https://file.io/mock" {
		t.Fatalf("URL inesperada: %s", result)
	}
}

func mustArchive(t *testing.T) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "proyecto.zip")
	if err := os.WriteFile(path, []byte("zip-data"), 0o644); err != nil {
		t.Fatalf("no se pudo crear ZIP temporal: %v", err)
	}
	return path
}
