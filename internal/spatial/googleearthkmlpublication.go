package spatial

import (
	"github.com/boostao/vpro-wails/internal/vprocore"
)

import (
	"bytes"
	"context"
	"errors"
)

// Finalized text/map points only; this is not an owned desktop export API.
func publishGoogleEarthKML(ctx context.Context, requested, title string, points []googleEarthKMLPoint) (vprocore.ArtifactPublication, error) {
	return publishGoogleEarthKMLWithHooks(ctx, requested, title, points, vprocore.ArtifactPublicationHooks{})
}

func publishGoogleEarthKMLWithHooks(ctx context.Context, requested, title string, points []googleEarthKMLPoint, hooks vprocore.ArtifactPublicationHooks) (vprocore.ArtifactPublication, error) {
	return publishGoogleEarthKMLChecked(ctx, requested, title, points, nil, hooks)
}

func publishGoogleEarthKMLChecked(ctx context.Context, requested, title string, points []googleEarthKMLPoint, validateSource func() error, hooks vprocore.ArtifactPublicationHooks) (vprocore.ArtifactPublication, error) {
	points = append([]googleEarthKMLPoint(nil), points...)
	return vprocore.PublishArtifactChecked(ctx, requested, vprocore.ArtifactPublicationFormat{
		Label:        "Google Earth KML publication",
		Artifact:     "KML",
		CommitLabel:  "Google Earth KML",
		StagePattern: ".vpro-google-earth-kml-*",
		Encode: func(ctx context.Context) ([]byte, error) {
			return prepareGoogleEarthKML(ctx, title, points)
		},
		Validate: func(ctx context.Context, staged []byte) error {
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
