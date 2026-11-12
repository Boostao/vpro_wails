package interchange

import (
	"github.com/boostao/vpro-wails/internal/vprocore"
)

import "context"

type tableCSVPublication = vprocore.ArtifactPublication
type tableCSVPublicationHooks = vprocore.ArtifactPublicationHooks

func publishTableCSVBundle(ctx context.Context, requested string, document tableCSVDocument) (tableCSVPublication, error) {
	return publishTableCSVBundleWithHooks(ctx, requested, document, tableCSVPublicationHooks{})
}

func publishTableCSVBundleWithHooks(ctx context.Context, requested string, document tableCSVDocument, hooks tableCSVPublicationHooks) (tableCSVPublication, error) {
	return publishTableCSVBundleChecked(ctx, requested, document, nil, hooks)
}

func publishTableCSVBundleChecked(ctx context.Context, requested string, document tableCSVDocument, validateSource func() error, hooks tableCSVPublicationHooks) (tableCSVPublication, error) {
	document = snapshotTableCSVBundleDocument(document)
	return vprocore.PublishArtifactChecked(ctx, requested, vprocore.ArtifactPublicationFormat{
		Label:        "table CSV publication",
		Artifact:     "bundle",
		CommitLabel:  "table CSV bundle",
		StagePattern: ".vpro-table-csv-*",
		Encode: func(ctx context.Context) ([]byte, error) {
			return encodeTableCSVBundle(ctx, document)
		},
		Validate: func(ctx context.Context, encoded []byte) error {
			_, err := decodeTableCSVBundle(ctx, encoded, int64(len(encoded)))
			return err
		},
	}, validateSource, hooks)
}
