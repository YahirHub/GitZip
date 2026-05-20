package zip

import (
	"errors"
	"io"
)

// EncryptionMethod describes the encryption mode requested for an archive
// entry. This vendored fork intentionally keeps only StandardEncryption
// operational because gitzip needs compatibility with `unzip -P`.
type EncryptionMethod int

const (
	StandardEncryption EncryptionMethod = 1
	AES128Encryption   EncryptionMethod = 2
	AES192Encryption   EncryptionMethod = 3
	AES256Encryption   EncryptionMethod = 4
)

var (
	ErrDecryption     = errors.New("zip: decryption error")
	ErrPassword       = errors.New("zip: invalid password")
	ErrAuthentication = errors.New("zip: authentication failed")
	errAESDisabled    = errors.New("zip: AES encryption is not included in this vendored build")
)

func newDecryptionReader(_ *io.SectionReader, _ *File) (io.Reader, error) {
	return nil, errAESDisabled
}

func newEncryptionWriter(_ io.Writer, _ passwordFn, _ *fileWriter, _ byte) (io.Writer, error) {
	return nil, errAESDisabled
}

// IsEncrypted indicates whether this file's data is encrypted.
func (h *FileHeader) IsEncrypted() bool {
	return h.Flags&0x1 == 1
}

func (h *FileHeader) isAE2() bool {
	return false
}

func (h *FileHeader) writeWinZipExtra() {}

// SetEncryptionMethod sets the encryption method.
func (h *FileHeader) SetEncryptionMethod(enc EncryptionMethod) {
	h.encryption = enc
}

func (h *FileHeader) setEncryptionBit() {
	h.Flags |= 0x1
}

// SetPassword sets the password used for encryption/decryption.
func (h *FileHeader) SetPassword(password string) {
	if !h.IsEncrypted() {
		h.setEncryptionBit()
	}
	h.password = func() []byte {
		return []byte(password)
	}
}

// PasswordFn is a function that returns the password as a byte slice.
type passwordFn func() []byte

// Encrypt adds a file to the zip file using the provided name. In this
// vendored build only StandardEncryption is supported.
func (w *Writer) Encrypt(name string, password string, enc EncryptionMethod) (io.Writer, error) {
	if enc != StandardEncryption {
		return nil, errAESDisabled
	}
	fh := &FileHeader{
		Name:   name,
		Method: Deflate,
	}
	fh.SetPassword(password)
	fh.SetEncryptionMethod(enc)
	return w.CreateHeader(fh)
}
