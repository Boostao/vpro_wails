package main

import "context"

type tableCSVPublication = artifactPublication
type tableCSVPublicationHooks = artifactPublicationHooks

func publishTableCSVBundle(ctx context.Context, requested string, document tableCSVDocument) (tableCSVPublication, error) {
	return publishTableCSVBundleWithHooks(ctx, requested, document, tableCSVPublicationHooks{})
}

func publishTableCSVBundleWithHooks(ctx context.Context, requested string, document tableCSVDocument, hooks tableCSVPublicationHooks) (tableCSVPublication, error) {
	return publishTableCSVBundleChecked(ctx, requested, document, nil, hooks)
}

func publishTableCSVBundleChecked(ctx context.Context, requested string, document tableCSVDocument, validateSource func() error, hooks tableCSVPublicationHooks) (tableCSVPublication, error) {
	document = snapshotTableCSVBundleDocument(document)
	return publishArtifactChecked(ctx, requested, artifactPublicationFormat{
		label:        "table CSV publication",
		artifact:     "bundle",
		commitLabel:  "table CSV bundle",
		stagePattern: ".vpro-table-csv-*",
		encode: func(ctx context.Context) ([]byte, error) {
			return encodeTableCSVBundle(ctx, document)
		},
		validate: func(ctx context.Context, encoded []byte) error {
			_, err := decodeTableCSVBundle(ctx, encoded, int64(len(encoded)))
			return err
		},
	}, validateSource, hooks)
}
