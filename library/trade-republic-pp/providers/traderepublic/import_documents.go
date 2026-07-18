package traderepublic

import (
	"context"
	"fmt"
	"path/filepath"
	"sort"
	"time"

	"trade-republic-pp-cli/internal/transactions"
)

type DocumentImportRequest struct {
	SourceDirectory      string
	DestinationDirectory string
	Since                *time.Time
}

type DocumentImportResult struct {
	Documents []transactions.Document `json:"documents"`
	Warnings  []string                `json:"warnings,omitempty"`
}

// ImportDocuments scans an existing local directory without invoking pytr.
// Files are bounded, validated as PDFs, content-addressed by SHA-256, copied
// into the private destination, and enriched with best-effort statement text
// metadata.
func (a *Adapter) ImportDocuments(ctx context.Context, request DocumentImportRequest) (DocumentImportResult, error) {
	if request.SourceDirectory == "" {
		return DocumentImportResult{}, fmt.Errorf("source directory is required")
	}
	destination := request.DestinationDirectory
	if destination == "" {
		destination = a.documentsDirectory
	}
	if destination == "" {
		return DocumentImportResult{}, fmt.Errorf("destination directory is required")
	}
	ctx, cancel := a.commandContext(ctx)
	defer cancel()

	source, err := filepath.Abs(request.SourceDirectory)
	if err != nil {
		return DocumentImportResult{}, err
	}
	destination, err = filepath.Abs(destination)
	if err != nil {
		return DocumentImportResult{}, err
	}
	paths, err := findPDFs(source)
	if err != nil {
		return DocumentImportResult{}, err
	}
	if err := ensurePrivateDir(destination); err != nil {
		return DocumentImportResult{}, fmt.Errorf("create documents directory: %w", err)
	}

	result := DocumentImportResult{
		Documents: make([]transactions.Document, 0, len(paths)),
		Warnings:  make([]string, 0),
	}
	importedAt := a.now().UTC()
	for _, sourcePath := range paths {
		hash, err := hashPDF(sourcePath)
		if err != nil {
			result.Warnings = append(result.Warnings, fmt.Sprintf("skipped PDF %s: %v", filepath.Base(sourcePath), err))
			continue
		}
		text, extractionErr := extractPDFText(ctx, a.runner, a.pdfToTextCommand, sourcePath)
		metadata := StatementMetadata{DocumentType: "unknown"}
		if extractionErr == nil {
			metadata = ParseStatementMetadata(text, a.location)
		} else {
			result.Warnings = append(result.Warnings, fmt.Sprintf("registered PDF %s without statement text metadata: %v", filepath.Base(sourcePath), extractionErr))
		}
		if metadata.DocumentType == "unknown" {
			metadata.DocumentType = inferDocumentTypeFromFilename(filepath.Base(sourcePath))
		}
		if request.Since != nil && !metadata.OccurredAt.IsZero() && metadata.OccurredAt.Before(*request.Since) {
			continue
		}
		destinationPath := filepath.Join(destination, hash+".pdf")
		if err := persistPDF(sourcePath, destinationPath, hash); err != nil {
			return result, fmt.Errorf("persist PDF %s: %w", filepath.Base(sourcePath), err)
		}
		result.Documents = append(result.Documents, transactions.Document{
			ID:            hash,
			SHA256:        hash,
			Path:          destinationPath,
			Filename:      filepath.Base(sourcePath),
			DocumentType:  metadata.DocumentType,
			OccurredAt:    metadata.OccurredAt,
			ISIN:          metadata.ISIN,
			Source:        "local:statement_pdf",
			ImportedAt:    importedAt,
			ParserVersion: StatementParserVersion,
		})
	}
	sort.Slice(result.Documents, func(i, j int) bool { return result.Documents[i].SHA256 < result.Documents[j].SHA256 })
	return result, nil
}
