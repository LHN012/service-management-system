package deploy

import (
	"archive/tar"
	"archive/zip"
	"bufio"
	"compress/gzip"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

const maxExtractedBytes int64 = 5 << 30

func extractArchive(source, target, configuredFormat string) error {
	format, err := detectArchiveFormat(source, configuredFormat)
	if err != nil {
		return err
	}
	switch format {
	case "zip":
		return extractZip(source, target)
	case "tar", "tar.gz", "tgz":
		return extractTar(source, target, format == "tar.gz" || format == "tgz")
	default:
		return fmt.Errorf("unsupported archive format %q", format)
	}
}

func detectArchiveFormat(source, configured string) (string, error) {
	if configured != "" && configured != "auto" {
		return configured, nil
	}
	file, err := os.Open(source)
	if err != nil {
		return "", err
	}
	defer file.Close()
	header := make([]byte, 4)
	n, _ := io.ReadFull(file, header)
	if n >= 2 && header[0] == 0x50 && header[1] == 0x4b {
		return "zip", nil
	}
	if n >= 2 && header[0] == 0x1f && header[1] == 0x8b {
		return "tar.gz", nil
	}
	lower := strings.ToLower(source)
	switch {
	case strings.HasSuffix(lower, ".tar"):
		return "tar", nil
	case strings.HasSuffix(lower, ".tgz"):
		return "tgz", nil
	case strings.HasSuffix(lower, ".tar.gz"):
		return "tar.gz", nil
	case strings.HasSuffix(lower, ".zip"):
		return "zip", nil
	default:
		return "", fmt.Errorf("cannot detect archive format for %s", source)
	}
}

func extractZip(source, target string) error {
	reader, err := zip.OpenReader(source)
	if err != nil {
		return err
	}
	defer reader.Close()
	var total int64
	for _, entry := range reader.File {
		if isMetadataPath(entry.Name) {
			continue
		}
		destination, err := safeExtractPath(target, entry.Name)
		if err != nil {
			return err
		}
		if entry.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("archive symlink %q is not allowed", entry.Name)
		}
		if entry.FileInfo().IsDir() {
			if err := os.MkdirAll(destination, 0o755); err != nil {
				return err
			}
			continue
		}
		if !entry.Mode().IsRegular() {
			return fmt.Errorf("archive entry %q is not a regular file", entry.Name)
		}
		total += int64(entry.UncompressedSize64)
		if total > maxExtractedBytes {
			return fmt.Errorf("archive exceeds %d extracted bytes", maxExtractedBytes)
		}
		if err := os.MkdirAll(filepath.Dir(destination), 0o755); err != nil {
			return err
		}
		input, err := entry.Open()
		if err != nil {
			return err
		}
		output, err := os.OpenFile(destination, os.O_CREATE|os.O_EXCL|os.O_WRONLY, entry.Mode().Perm()&0o755)
		if err != nil {
			input.Close()
			return err
		}
		_, copyErr := io.Copy(output, io.LimitReader(input, maxExtractedBytes+1))
		closeErr := output.Close()
		input.Close()
		if copyErr != nil {
			return copyErr
		}
		if closeErr != nil {
			return closeErr
		}
	}
	return nil
}

func extractTar(source, target string, compressed bool) error {
	file, err := os.Open(source)
	if err != nil {
		return err
	}
	defer file.Close()
	var input io.Reader = bufio.NewReader(file)
	if compressed {
		gz, err := gzip.NewReader(input)
		if err != nil {
			return err
		}
		defer gz.Close()
		input = gz
	}
	reader := tar.NewReader(input)
	var total int64
	for {
		header, err := reader.Next()
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return err
		}
		if isMetadataPath(header.Name) {
			continue
		}
		destination, err := safeExtractPath(target, header.Name)
		if err != nil {
			return err
		}
		switch header.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(destination, os.FileMode(header.Mode)&0o755); err != nil {
				return err
			}
		case tar.TypeReg, tar.TypeRegA:
			total += header.Size
			if total > maxExtractedBytes {
				return fmt.Errorf("archive exceeds %d extracted bytes", maxExtractedBytes)
			}
			if err := os.MkdirAll(filepath.Dir(destination), 0o755); err != nil {
				return err
			}
			output, err := os.OpenFile(destination, os.O_CREATE|os.O_EXCL|os.O_WRONLY, os.FileMode(header.Mode)&0o755)
			if err != nil {
				return err
			}
			_, copyErr := io.CopyN(output, reader, header.Size)
			closeErr := output.Close()
			if copyErr != nil {
				return copyErr
			}
			if closeErr != nil {
				return closeErr
			}
		default:
			return fmt.Errorf("archive entry %q has unsafe type %d", header.Name, header.Typeflag)
		}
	}
}

func safeExtractPath(root, name string) (string, error) {
	clean := filepath.Clean(filepath.FromSlash(name))
	if clean == "." || filepath.IsAbs(clean) || clean == ".." || strings.HasPrefix(clean, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("archive entry %q escapes extraction root", name)
	}
	destination := filepath.Join(root, clean)
	if err := ensureWithin(root, destination); err != nil {
		return "", fmt.Errorf("archive entry %q: %w", name, err)
	}
	return destination, nil
}

func isMetadataPath(name string) bool {
	parts := strings.FieldsFunc(filepath.ToSlash(name), func(r rune) bool { return r == '/' })
	for _, part := range parts {
		if part == "__MACOSX" || part == ".DS_Store" {
			return true
		}
	}
	return false
}
