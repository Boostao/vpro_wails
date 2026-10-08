package main

import (
	"bytes"
	"context"
	"encoding/xml"
	"fmt"
	"html"
	"math"
	"strconv"
	"unicode/utf8"
)

const googleEarthKMLNamespace = "http://earth.google.com/kml/2.1"

// These are explicitly prepared text and map coordinates, not raw database
// cells. Longitude is already east-positive; this boundary never negates it.
type googleEarthKMLPoint struct {
	Name, Description   string
	Longitude, Latitude float64
}

type googleEarthKMLPlacemark struct {
	Name        string `xml:"name"`
	Description string `xml:"description"`
	Point       struct {
		Coordinates string `xml:"coordinates"`
	} `xml:"Point"`
}

func validateGoogleEarthXMLText(value string) error {
	if !utf8.ValidString(value) {
		return fmt.Errorf("invalid UTF-8; XML text is not repaired")
	}
	for _, char := range value {
		if char != '\t' && char != '\n' && char != '\r' &&
			!(char >= 0x20 && char <= 0xd7ff || char >= 0xe000 && char <= 0xfffd || char >= 0x10000 && char <= 0x10ffff) {
			return fmt.Errorf("character U+%04X is not XML 1.0 text", char)
		}
	}
	return nil
}

func validateGoogleEarthKMLCoordinates(longitude, latitude float64) error {
	if math.IsNaN(longitude) || math.IsInf(longitude, 0) || longitude < -180 || longitude > 180 ||
		math.IsNaN(latitude) || math.IsInf(latitude, 0) || latitude < -90 || latitude > 90 {
		return fmt.Errorf("requires finite longitude [-180,180] and latitude [-90,90]; values are not clamped")
	}
	return nil
}

func prepareGoogleEarthKML(ctx context.Context, title string, points []googleEarthKMLPoint) ([]byte, error) {
	points = append([]googleEarthKMLPoint(nil), points...)
	if ctx == nil {
		return nil, fmt.Errorf("Google Earth KML preparation requires a context")
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if err := validateGoogleEarthXMLText(title); err != nil {
		return nil, fmt.Errorf("Google Earth document name: %w", err)
	}
	for i, point := range points {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		for _, field := range []struct{ name, value string }{{"name", point.Name}, {"description", point.Description}} {
			if err := validateGoogleEarthXMLText(field.value); err != nil {
				return nil, fmt.Errorf("Google Earth point %d %s: %w", i, field.name, err)
			}
		}
		if err := validateGoogleEarthKMLCoordinates(point.Longitude, point.Latitude); err != nil {
			return nil, fmt.Errorf("Google Earth point %d %w", i, err)
		}
	}
	var buffer bytes.Buffer
	buffer.WriteString(xml.Header)
	encoder := xml.NewEncoder(&buffer)
	root := xml.StartElement{Name: xml.Name{Space: googleEarthKMLNamespace, Local: "kml"}}
	document := xml.StartElement{Name: xml.Name{Local: "Document"}}
	if err := encoder.EncodeToken(root); err != nil {
		return nil, fmt.Errorf("encode Google Earth root: %w", err)
	}
	if err := encoder.EncodeToken(document); err != nil {
		return nil, fmt.Errorf("encode Google Earth document: %w", err)
	}
	if err := encoder.EncodeElement(title, xml.StartElement{Name: xml.Name{Local: "name"}}); err != nil {
		return nil, fmt.Errorf("encode Google Earth document name: %w", err)
	}
	for i, point := range points {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		// Earth descriptions can interpret HTML after XML decoding.
		mark := googleEarthKMLPlacemark{Name: point.Name, Description: html.EscapeString(point.Description + ".")}
		mark.Point.Coordinates = strconv.FormatFloat(point.Longitude, 'f', -1, 64) + "," +
			strconv.FormatFloat(point.Latitude, 'f', -1, 64) + ",0"
		if err := encoder.EncodeElement(mark, xml.StartElement{Name: xml.Name{Local: "Placemark"}}); err != nil {
			return nil, fmt.Errorf("encode Google Earth point %d: %w", i, err)
		}
	}
	if err := encoder.EncodeToken(document.End()); err != nil {
		return nil, fmt.Errorf("finish Google Earth document: %w", err)
	}
	if err := encoder.EncodeToken(root.End()); err != nil {
		return nil, fmt.Errorf("finish Google Earth root: %w", err)
	}
	if err := encoder.Flush(); err != nil {
		return nil, fmt.Errorf("flush Google Earth XML: %w", err)
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return append([]byte(nil), buffer.Bytes()...), nil
}
