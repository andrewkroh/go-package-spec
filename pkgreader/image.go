package pkgreader

import (
	"crypto/sha256"
	"encoding/hex"
	"image"
	"io"
	"io/fs"
	"path"
	"strings"

	_ "image/jpeg" // Register JPEG decoder.
	_ "image/png"  // Register PNG decoder.
)

// ImageFile represents an image file with metadata extracted from its contents.
type ImageFile struct {
	Width    int    // pixels
	Height   int    // pixels
	ByteSize int64  // file size in bytes
	SHA256   string // hex-encoded SHA-256 hash of file contents
	path     string
}

// Path returns the file path relative to the package root.
func (img *ImageFile) Path() string {
	return img.path
}

// supportedImageExt reports whether the file extension is a supported image format.
func supportedImageExt(name string) bool {
	lower := strings.ToLower(name)
	return strings.HasSuffix(lower, ".png") ||
		strings.HasSuffix(lower, ".jpg") ||
		strings.HasSuffix(lower, ".jpeg") ||
		strings.HasSuffix(lower, ".svg")
}

func readImages(fsys fs.FS, dir string) (map[string]*ImageFile, error) {
	entries, err := fs.ReadDir(fsys, dir)
	if err != nil {
		if isNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	var result map[string]*ImageFile
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if !supportedImageExt(name) {
			continue
		}

		filePath := path.Join(dir, name)

		info, err := entry.Info()
		if err != nil {
			return nil, err
		}

		img := &ImageFile{
			ByteSize: info.Size(),
			path:     filePath,
		}

		// Compute SHA-256 hash.
		hash, err := hashFile(fsys, filePath)
		if err != nil {
			return nil, err
		}
		img.SHA256 = hash

		// Decode dimensions for raster formats (not SVG).
		if !strings.HasSuffix(strings.ToLower(name), ".svg") {
			w, h, decodeErr := decodeImageDimensions(fsys, filePath)
			if decodeErr == nil {
				img.Width = w
				img.Height = h
			}
		}

		if result == nil {
			result = make(map[string]*ImageFile)
		}
		result[name] = img
	}

	return result, nil
}

func hashFile(fsys fs.FS, filePath string) (string, error) {
	f, err := fsys.Open(filePath)
	if err != nil {
		return "", err
	}
	defer f.Close()

	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

func decodeImageDimensions(fsys fs.FS, filePath string) (width, height int, err error) {
	f, err := fsys.Open(filePath)
	if err != nil {
		return 0, 0, err
	}
	defer f.Close()

	cfg, _, err := image.DecodeConfig(f)
	if err != nil {
		return 0, 0, err
	}
	return cfg.Width, cfg.Height, nil
}
