package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
)

const ownedTableCSVArchiveFormat = "vpro-owned-table-csv"

// This envelope is separate from the unchanged version1 table document.
type ownedTableCSVArchiveManifest struct {
	Format                     string           `json:"format"`
	Version                    int              `json:"version"`
	DescriptionMetadataPresent bool             `json:"descriptionMetadataPresent"`
	Document                   TableCSVManifest `json:"document"`
}

type ownedTableCSVArchive struct {
	DescriptionMetadataPresent bool
	Document                   tableCSVDocument
}

func encodeOwnedTableCSVArchive(ctx context.Context, archive ownedTableCSVArchive) ([]byte, error) {
	archive.Document = snapshotTableCSVBundleDocument(archive.Document)
	if err := validateOwnedTableCSVArchive(ctx, archive); err != nil {
		return nil, err
	}
	manifest, err := json.Marshal(ownedTableCSVArchiveManifest{
		Format: ownedTableCSVArchiveFormat, Version: 1,
		DescriptionMetadataPresent: archive.DescriptionMetadataPresent,
		Document:                   archive.Document.Manifest,
	})
	if err != nil {
		return nil, fmt.Errorf("owned table CSV archive manifest: %w", err)
	}
	if err := validateTableCSVBundleJSON(ctx, manifest, "owned-manifest"); err != nil {
		return nil, fmt.Errorf("owned table CSV archive manifest: %w", err)
	}
	return encodeTableCSVBundleMembers(ctx, archive.Document.Data, manifest)
}

func decodeOwnedTableCSVArchive(ctx context.Context, encoded []byte, byteBudget int64) (ownedTableCSVArchive, error) {
	var zero ownedTableCSVArchive
	members, err := decodeTableCSVBundleMembers(ctx, encoded, byteBudget)
	if err != nil {
		return zero, err
	}
	raw := members["manifest.json"]
	if err := validateTableCSVBundleJSON(ctx, raw, "owned-manifest"); err != nil {
		return zero, fmt.Errorf("owned table CSV archive manifest: %w", err)
	}
	var manifest ownedTableCSVArchiveManifest
	if err := json.Unmarshal(raw, &manifest); err != nil {
		return zero, fmt.Errorf("owned table CSV archive manifest decode: %w", err)
	}
	if manifest.Format != ownedTableCSVArchiveFormat || manifest.Version != 1 {
		return zero, errors.New("owned table CSV archive format/version is unsupported")
	}
	archive := ownedTableCSVArchive{
		DescriptionMetadataPresent: manifest.DescriptionMetadataPresent,
		Document: tableCSVDocument{
			Manifest: manifest.Document, Data: members["table.csv"],
		},
	}
	if err := validateOwnedTableCSVArchive(ctx, archive); err != nil {
		return zero, err
	}
	return archive, nil
}

func validateOwnedTableCSVArchive(ctx context.Context, archive ownedTableCSVArchive) error {
	if _, err := decodeTableCSV(ctx, archive.Document); err != nil {
		return fmt.Errorf("owned table CSV archive document: %w", err)
	}
	if !archive.DescriptionMetadataPresent && len(archive.Document.Manifest.Descriptions) != 0 {
		return errors.New("owned table CSV archive cannot contain Description candidates when metadata is absent")
	}
	return ctx.Err()
}

func publishOwnedTableCSVArchiveChecked(ctx context.Context, requested string, review ownedTableCSVReview, validateSource func() error, hooks tableCSVPublicationHooks) (tableCSVPublication, error) {
	archive := ownedTableCSVArchive{
		DescriptionMetadataPresent: review.DescriptionMetadataPresent,
		Document:                   snapshotTableCSVBundleDocument(review.Document),
	}
	return publishArtifactChecked(ctx, requested, artifactPublicationFormat{
		label: "owned table CSV archive publication", artifact: "archive",
		commitLabel: "owned table CSV archive", stagePattern: ".vpro-owned-table-csv-*",
		encode: func(ctx context.Context) ([]byte, error) {
			return encodeOwnedTableCSVArchive(ctx, archive)
		},
		validate: func(ctx context.Context, encoded []byte) error {
			_, err := decodeOwnedTableCSVArchive(ctx, encoded, int64(len(encoded)))
			return err
		},
	}, validateSource, hooks)
}
