package menu

import (
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/google/uuid"
)

const (
	defaultMenuUploadDir = "uploads/menu-items"
	menuUploadURLPrefix  = "/uploads/menu-items/"
	maxMenuImageBytes    = 5 << 20
)

var (
	ErrInvalidImageFormat = errors.New("unsupported image format, allowed formats are JPG, PNG, and WEBP")
	ErrImageTooLarge      = errors.New("image size must be 5MB or smaller")
)

func UploadRootDir() string {
	dir := strings.TrimSpace(os.Getenv("MENU_UPLOAD_DIR"))
	if dir == "" {
		dir = defaultMenuUploadDir
	}

	return filepath.Clean(dir)
}

func ensureUploadDir() error {
	return os.MkdirAll(UploadRootDir(), 0o755)
}

func ReadUploadedImage(fileHeader *multipart.FileHeader) (*UploadedImage, error) {
	if fileHeader == nil {
		return nil, nil
	}
	if fileHeader.Size > maxMenuImageBytes {
		return nil, ErrImageTooLarge
	}

	file, err := fileHeader.Open()
	if err != nil {
		return nil, err
	}
	defer file.Close()

	data, err := io.ReadAll(io.LimitReader(file, maxMenuImageBytes+1))
	if err != nil {
		return nil, err
	}
	if len(data) == 0 {
		return nil, nil
	}
	if len(data) > maxMenuImageBytes {
		return nil, ErrImageTooLarge
	}

	contentType := http.DetectContentType(data[:min(512, len(data))])
	if _, err := imageExtensionFromContentType(contentType); err != nil {
		return nil, err
	}

	return &UploadedImage{
		Filename:    strings.TrimSpace(fileHeader.Filename),
		ContentType: contentType,
		Data:        data,
	}, nil
}

func saveUploadedImage(image *UploadedImage) (string, error) {
	if image == nil || len(image.Data) == 0 {
		return "", nil
	}

	extension, err := imageExtensionFromContentType(image.ContentType)
	if err != nil {
		return "", err
	}
	if err := ensureUploadDir(); err != nil {
		return "", err
	}

	fileName := fmt.Sprintf("%s%s", uuid.NewString(), extension)
	filePath := filepath.Join(UploadRootDir(), fileName)
	if err := os.WriteFile(filePath, image.Data, 0o644); err != nil {
		return "", err
	}

	return menuUploadURLPrefix + fileName, nil
}

func deleteManagedImage(imageURL string) error {
	filePath, ok := managedImagePath(imageURL)
	if !ok {
		return nil
	}

	if err := os.Remove(filePath); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}

	return nil
}

func managedImagePath(imageURL string) (string, bool) {
	trimmed := strings.TrimSpace(imageURL)
	if !strings.HasPrefix(trimmed, menuUploadURLPrefix) {
		return "", false
	}

	fileName := strings.TrimPrefix(trimmed, menuUploadURLPrefix)
	if fileName == "" || fileName != filepath.Base(fileName) {
		return "", false
	}

	return filepath.Join(UploadRootDir(), fileName), true
}

func imageExtensionFromContentType(contentType string) (string, error) {
	switch strings.ToLower(strings.TrimSpace(contentType)) {
	case "image/jpeg":
		return ".jpg", nil
	case "image/png":
		return ".png", nil
	case "image/webp":
		return ".webp", nil
	default:
		return "", ErrInvalidImageFormat
	}
}

func min(left, right int) int {
	if left < right {
		return left
	}
	return right
}
