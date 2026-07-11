package cli

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"

	"github.com/yasyf/cc-present/internal/assets"
	"github.com/yasyf/cc-present/internal/doc"
)

// uploader turns a local image file path into an asset:<sha256> reference.
type uploader func(path string) (string, error)

// isLocalSrc reports whether an image src names a local file to inline, as
// opposed to an already-resolved https:, asset:, or data: reference.
func isLocalSrc(src string) bool {
	return !strings.HasPrefix(src, "https://") &&
		!strings.HasPrefix(src, "asset:") &&
		!strings.HasPrefix(src, "data:")
}

// readImageFile reads a local image, failing loudly when it is missing, past
// the asset cap, or does not sniff as an image.
func readImageFile(path string) ([]byte, error) {
	//nolint:gosec // G304: reading the local image the document references is the inliner's purpose.
	b, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return nil, fmt.Errorf("image %q not found", path)
	}
	if err != nil {
		return nil, fmt.Errorf("read image %q: %w", path, err)
	}
	if len(b) > assets.MaxBytes {
		return nil, fmt.Errorf("image %q is %d bytes, exceeds %d", path, len(b), assets.MaxBytes)
	}
	if ct := http.DetectContentType(b); !strings.HasPrefix(ct, "image/") {
		return nil, fmt.Errorf("image %q sniffs as %s, not an image", path, ct)
	}
	return b, nil
}

// httpUploader stores each image on the daemon's asset endpoint and returns its
// asset:<sha256> reference.
func httpUploader(port int) uploader {
	return func(path string) (string, error) {
		b, err := readImageFile(path)
		if err != nil {
			return "", err
		}
		resp, err := http.Post(
			fmt.Sprintf("http://127.0.0.1:%d/api/assets", port), "application/octet-stream", bytes.NewReader(b),
		)
		if err != nil {
			return "", err
		}
		defer func() { _ = resp.Body.Close() }()
		if resp.StatusCode != http.StatusOK {
			msg, _ := io.ReadAll(resp.Body)
			return "", fmt.Errorf("upload %q: %s: %s", path, resp.Status, strings.TrimSpace(string(msg)))
		}
		var out struct {
			Asset string `json:"asset"`
		}
		if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
			return "", err
		}
		return out.Asset, nil
	}
}

// localUploader content-addresses each image without contacting the daemon, for
// dry-run validation.
func localUploader(path string) (string, error) {
	b, err := readImageFile(path)
	if err != nil {
		return "", err
	}
	return "asset:" + assets.SHA(b), nil
}

// inlineImages rewrites every image block with a local-file src to its
// asset:<sha256> reference, recursing one level into cards. Image bytes are read
// and stored through up before the document is validated or appended.
func inlineImages(blocks []doc.Block, up uploader) error {
	for _, b := range blocks {
		if err := doc.RewriteAssetSrcs(b, func(src string) (string, error) {
			if !isLocalSrc(src) {
				return src, nil
			}
			return up(src)
		}); err != nil {
			return err
		}
	}
	return nil
}
