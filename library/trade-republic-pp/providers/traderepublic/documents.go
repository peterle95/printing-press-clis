package traderepublic

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"trade-republic-pp-cli/internal/transactions"
)

const (
	maxPDFBytes int64 = 64 << 20
	maxPDFCount       = 10_000
)

// DocumentAlias keeps adapter.go independent of the document implementation
// while remaining exactly the shared normalized document type.
type DocumentAlias = transactions.Document

func makeDocuments(documents []DocumentAlias) []transactions.Document { return documents }

func (a *Adapter) syncDocuments(
	ctx context.Context,
	stage string,
	destination string,
	lastDays int,
	since *time.Time,
	importedAt time.Time,
) ([]DocumentAlias, []string, error) {
	downloadDirectory := filepath.Join(stage, "document-download")
	if err := ensurePrivateDir(downloadDirectory); err != nil {
		return nil, nil, err
	}
	if err := a.runHelper(ctx, downloadDirectory, a.argv(
		"dl_docs",
		"--lang", "en",
		"--date-with-time",
		"--no-decimal-localization",
		"--sort",
		"--last_days", strconv.Itoa(lastDays),
		"--workers", "4",
		"--universal",
		"--no-store-event-database",
		"--no-scan-for-duplicates",
		"--no-dump-raw-data",
		"--no-export-transactions",
		"--export-format", "json",
		"--flat",
		downloadDirectory,
	)); err != nil {
		return nil, nil, fmt.Errorf("pytr document download failed: %w", err)
	}

	paths, err := findPDFs(downloadDirectory)
	if err != nil {
		return nil, nil, err
	}
	destination, err = filepath.Abs(destination)
	if err != nil {
		return nil, nil, err
	}
	if err := ensurePrivateDir(destination); err != nil {
		return nil, nil, fmt.Errorf("create documents directory: %w", err)
	}

	documents := make([]DocumentAlias, 0, len(paths))
	warnings := make([]string, 0)
	for _, sourcePath := range paths {
		hash, err := hashPDF(sourcePath)
		if err != nil {
			warnings = append(warnings, fmt.Sprintf("skipped PDF %s: %v", filepath.Base(sourcePath), err))
			continue
		}
		text, extractionErr := extractPDFText(ctx, a.runner, a.pdfToTextCommand, sourcePath)
		metadata := StatementMetadata{DocumentType: "unknown"}
		if extractionErr == nil {
			metadata = ParseStatementMetadata(text, a.location)
		} else {
			warnings = append(warnings, fmt.Sprintf("registered PDF %s without statement text metadata: %v", filepath.Base(sourcePath), extractionErr))
		}
		if metadata.DocumentType == "unknown" {
			metadata.DocumentType = inferDocumentTypeFromFilename(filepath.Base(sourcePath))
		}
		if since != nil && !metadata.OccurredAt.IsZero() && metadata.OccurredAt.Before(*since) {
			continue
		}
		destinationPath := filepath.Join(destination, hash+".pdf")
		if err := persistPDF(sourcePath, destinationPath, hash); err != nil {
			return nil, warnings, fmt.Errorf("persist PDF %s: %w", filepath.Base(sourcePath), err)
		}
		documents = append(documents, DocumentAlias{
			ID:            hash,
			SHA256:        hash,
			Path:          destinationPath,
			Filename:      filepath.Base(sourcePath),
			DocumentType:  metadata.DocumentType,
			OccurredAt:    metadata.OccurredAt,
			ISIN:          metadata.ISIN,
			Source:        "pytr:dl_docs",
			ImportedAt:    importedAt,
			ParserVersion: StatementParserVersion,
		})
	}
	sort.Slice(documents, func(i, j int) bool { return documents[i].SHA256 < documents[j].SHA256 })
	return documents, warnings, nil
}

func findPDFs(root string) ([]string, error) {
	paths := make([]string, 0)
	err := filepath.WalkDir(root, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.Type()&os.ModeSymlink != 0 {
			return nil
		}
		if entry.IsDir() || !strings.EqualFold(filepath.Ext(entry.Name()), ".pdf") {
			return nil
		}
		if len(paths) >= maxPDFCount {
			return fmt.Errorf("document download exceeds %d PDFs", maxPDFCount)
		}
		paths = append(paths, path)
		return nil
	})
	if err != nil {
		return nil, err
	}
	sort.Strings(paths)
	return paths, nil
}

func hashPDF(path string) (string, error) {
	file, err := openBoundedRegularFile(path, maxPDFBytes)
	if err != nil {
		return "", err
	}
	defer file.Close()
	header := make([]byte, 5)
	if _, err := io.ReadFull(file, header); err != nil {
		return "", fmt.Errorf("read PDF header: %w", err)
	}
	if string(header) != "%PDF-" {
		return "", fmt.Errorf("file does not have a PDF header")
	}
	if _, err := file.Seek(0, io.SeekStart); err != nil {
		return "", err
	}
	hasher := sha256.New()
	if _, err := io.Copy(hasher, io.LimitReader(file, maxPDFBytes+1)); err != nil {
		return "", err
	}
	return hex.EncodeToString(hasher.Sum(nil)), nil
}

func persistPDF(source, destination, expectedHash string) error {
	if info, err := os.Lstat(destination); err == nil {
		if !info.Mode().IsRegular() {
			return fmt.Errorf("destination is not a regular file")
		}
		existingHash, err := hashPDF(destination)
		if err != nil {
			return err
		}
		if existingHash != expectedHash {
			return fmt.Errorf("existing content hash does not match its filename")
		}
		return nil
	} else if !os.IsNotExist(err) {
		return err
	}

	input, err := openBoundedRegularFile(source, maxPDFBytes)
	if err != nil {
		return err
	}
	defer input.Close()
	temporary, err := os.CreateTemp(filepath.Dir(destination), ".document-*.pdf")
	if err != nil {
		return err
	}
	temporaryPath := temporary.Name()
	defer os.Remove(temporaryPath)
	if err := temporary.Chmod(0o600); err != nil {
		_ = temporary.Close()
		return err
	}
	_, copyErr := io.Copy(temporary, io.LimitReader(input, maxPDFBytes+1))
	if copyErr == nil {
		copyErr = temporary.Sync()
	}
	if closeErr := temporary.Close(); copyErr == nil {
		copyErr = closeErr
	}
	if copyErr != nil {
		return copyErr
	}
	return os.Rename(temporaryPath, destination)
}
