package spatial

import (
	"github.com/boostao/vpro-wails/internal/vprocore"
)

import (
	"context"
	"fmt"
)

func googleEarthKMLText(cell vprocore.ProjectMetadataCell) (string, error) {
	value, err := vprocore.MetadataCellValue(cell)
	if err != nil {
		return "", err
	}
	switch cell.Storage {
	case "null":
		// The source concatenates with non-NULL literals using VBA ampersand.
		return "", nil
	case "text":
		text := value.(string)
		if err := validateGoogleEarthXMLText(text); err != nil {
			return "", err
		}
		return text, nil
	default:
		return "", fmt.Errorf("KML text formatting for storage %q is unavailable; no implicit conversion", cell.Storage)
	}
}

func googleEarthKMLNumber(cell vprocore.ProjectMetadataCell) (float64, error) {
	value, err := vprocore.MetadataCellValue(cell)
	if err != nil {
		return 0, err
	}
	switch number := value.(type) {
	case int64:
		return float64(number), nil
	case float64:
		return number, nil
	default:
		return 0, fmt.Errorf("KML coordinates require original numeric storage, not %q", cell.Storage)
	}
}

// Rows come from the private direct-Env projection; this does not authorize a
// database read. Longitude is its already-negated value, not StoredLongitude.
func planGoogleEarthKMLPoints(ctx context.Context, rows []googleEarthLocation) ([]googleEarthKMLPoint, error) {
	rows = append([]googleEarthLocation(nil), rows...)
	for i := range rows {
		rows[i].PlotNumber = vprocore.CloneSiteUnitCell(rows[i].PlotNumber)
		rows[i].Longitude = vprocore.CloneSiteUnitCell(rows[i].Longitude)
		rows[i].Latitude = vprocore.CloneSiteUnitCell(rows[i].Latitude)
		rows[i].Description = vprocore.CloneSiteUnitCell(rows[i].Description)
	}
	if ctx == nil {
		return nil, fmt.Errorf("Google Earth KML points require a context")
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	points := make([]googleEarthKMLPoint, 0, len(rows))
	for i, row := range rows {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		fail := func(field string, err error) ([]googleEarthKMLPoint, error) {
			return nil, fmt.Errorf("Google Earth point %d Env row %q membership %q %s: %w", i, row.EnvRowID, row.MembershipRowID, field, err)
		}
		name, err := googleEarthKMLText(row.PlotNumber)
		if err != nil {
			return fail("PlotNumber", err)
		}
		description, err := googleEarthKMLText(row.Description)
		if err != nil {
			return fail("description", err)
		}
		longitude, err := googleEarthKMLNumber(row.Longitude)
		if err != nil {
			return fail("Longitude", err)
		}
		latitude, err := googleEarthKMLNumber(row.Latitude)
		if err != nil {
			return fail("Latitude", err)
		}
		if err := validateGoogleEarthKMLCoordinates(longitude, latitude); err != nil {
			return fail("coordinates", err)
		}
		points = append(points, googleEarthKMLPoint{Name: name, Description: description, Longitude: longitude, Latitude: latitude})
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return points, nil
}
