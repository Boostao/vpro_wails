package main

import (
	"bytes"
	"context"
	"errors"
)

// Finalized text/map points only; this is not an owned desktop export API.
func publishGoogleEarthKML(ctx context.Context, requested, title string, points []googleEarthKMLPoint) (artifactPublication, error) {
	return publishGoogleEarthKMLWithHooks(ctx, requested, title, points, artifactPublicationHooks{})
}

func publishGoogleEarthKMLWithHooks(ctx context.Context, requested, title string, points []googleEarthKMLPoint, hooks artifactPublicationHooks) (artifactPublication, error) {
	return publishGoogleEarthKMLChecked(ctx, requested, title, points, nil, hooks)
}

func publishGoogleEarthKMLChecked(ctx context.Context, requested, title string, points []googleEarthKMLPoint, validateSource func() error, hooks artifactPublicationHooks) (artifactPublication, error) {
	points = append([]googleEarthKMLPoint(nil), points...)
	return publishArtifactChecked(ctx, requested, artifactPublicationFormat{
		label:        "Google Earth KML publication",
		artifact:     "KML",
		commitLabel:  "Google Earth KML",
		stagePattern: ".vpro-google-earth-kml-*",
		encode: func(ctx context.Context) ([]byte, error) {
			return prepareGoogleEarthKML(ctx, title, points)
		},
		validate: func(ctx context.Context, staged []byte) error {
			expected, err := prepareGoogleEarthKML(ctx, title, points)
			if err != nil {
				return err
			}
			if !bytes.Equal(staged, expected) {
				return errors.New("staged KML differs from finalized source points")
			}
			return nil
		},
	}, validateSource, hooks)
}
