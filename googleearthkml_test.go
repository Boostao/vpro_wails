package main

import (
	"bytes"
	"context"
	"encoding/xml"
	"errors"
	"html"
	"io"
	"math"
	"reflect"
	"strings"
	"testing"
)

type observedGoogleEarthKML struct {
	XMLName  xml.Name `xml:"kml"`
	Document struct {
		Name  string `xml:"name"`
		Marks []struct {
			Name        string `xml:"name"`
			Description string `xml:"description"`
			Point       struct {
				Coordinates string `xml:"coordinates"`
			} `xml:"Point"`
		} `xml:"Placemark"`
	} `xml:"Document"`
}

func TestGoogleEarthKMLExactXMLShapeLiteralTextAndCoordinates(t *testing.T) {
	points := []googleEarthKMLPoint{
		{Name: "  A<&\" \U0001f332\r\n", Description: "x]]><img src=\"https://example.invalid/pixel\">&\r\n", Longitude: -123.25, Latitude: 54.75},
		{Name: "  A<&\" \U0001f332\r\n", Description: "", Longitude: math.Copysign(0, -1), Latitude: 90},
		{Name: "edge", Description: ".", Longitude: 180, Latitude: -90},
	}
	original := append([]googleEarthKMLPoint(nil), points...)
	title := " Title<&\t\r\n\U0001f332 "
	data, err := prepareGoogleEarthKML(context.Background(), title, points)
	if err != nil || !bytes.HasPrefix(data, []byte(xml.Header)) {
		t.Fatal("complete explicit UTF-8 XML missing", err)
	}
	var observed observedGoogleEarthKML
	if err := xml.Unmarshal(data, &observed); err != nil {
		t.Fatal(err)
	}
	if observed.XMLName.Space != googleEarthKMLNamespace || observed.Document.Name != title || len(observed.Document.Marks) != 3 {
		t.Fatalf("XML namespace/title/duplicates differ: %+v", observed)
	}
	for i, mark := range observed.Document.Marks {
		if mark.Name != points[i].Name || html.UnescapeString(mark.Description) != points[i].Description+"." ||
			strings.Contains(mark.Description, "<img") {
			t.Fatalf("literal XML/HTML text was repaired or interpreted: %+v", mark)
		}
	}
	for i, expected := range []string{"-123.25,54.75,0", "-0,90,0", "180,-90,0"} {
		if observed.Document.Marks[i].Point.Coordinates != expected {
			t.Fatalf("exact locale-neutral coordinates %d differ", i)
		}
	}
	decoder := xml.NewDecoder(bytes.NewReader(data))
	for {
		token, err := decoder.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatal(err)
		}
		if element, ok := token.(xml.StartElement); ok {
			if element.Name.Space != googleEarthKMLNamespace {
				t.Fatal("child namespace differs", element.Name)
			}
			switch element.Name.Local {
			case "kml", "Document", "name", "Placemark", "description", "Point", "coordinates":
			default:
				t.Fatal("unexpected resource/style/link or other element", element.Name)
			}
		}
	}
	if !reflect.DeepEqual(points, original) {
		t.Fatal("KML preparation mutated caller input")
	}
	again, err := prepareGoogleEarthKML(context.Background(), title, points)
	if err != nil || !bytes.Equal(data, again) {
		t.Fatal("deterministic bytes differ", err)
	}
	data[0] = 'X'
	if again[0] != '<' {
		t.Fatal("output byte slices aliased")
	}
}

func TestGoogleEarthKMLRejectsInvalidXMLTextAndMapCoordinates(t *testing.T) {
	point := googleEarthKMLPoint{Name: "name", Description: "description", Longitude: -123, Latitude: 54}
	for _, invalid := range []string{"\x00", "\x01", "\x0b", "\x1f", "\ufffe", "\uffff", string([]byte{0xff}), string([]byte{0xed, 0xa0, 0x80})} {
		for _, field := range []string{"title", "name", "description"} {
			t.Run(field, func(t *testing.T) {
				current, title := point, "title"
				switch field {
				case "title":
					title = invalid
				case "name":
					current.Name = invalid
				default:
					current.Description = invalid
				}
				if data, err := prepareGoogleEarthKML(context.Background(), title, []googleEarthKMLPoint{current}); err == nil || data != nil {
					t.Fatal("invalid Unicode/XML text returned partial success", err)
				}
			})
		}
	}
	for _, value := range []float64{math.NaN(), math.Inf(1), math.Inf(-1), math.Nextafter(180, 181), math.Nextafter(-180, -181)} {
		current := point
		current.Longitude = value
		if data, err := prepareGoogleEarthKML(context.Background(), "title", []googleEarthKMLPoint{current}); err == nil || data != nil {
			t.Fatal("invalid longitude returned partial/clamped success", value, err)
		}
	}
	for _, value := range []float64{math.NaN(), math.Inf(1), math.Inf(-1), math.Nextafter(90, 91), math.Nextafter(-90, -91)} {
		current := point
		current.Latitude = value
		if data, err := prepareGoogleEarthKML(context.Background(), "title", []googleEarthKMLPoint{current}); err == nil || data != nil {
			t.Fatal("invalid latitude returned partial/clamped success", value, err)
		}
	}
	data, err := prepareGoogleEarthKML(context.Background(), "", nil)
	var observed observedGoogleEarthKML
	if err != nil || xml.Unmarshal(data, &observed) != nil || observed.Document.Name != "" || len(observed.Document.Marks) != 0 {
		t.Fatal("explicit empty title/document lost", err)
	}
}

type googleEarthKMLCallbackContext struct {
	context.Context
	calls    int
	onCheck  func(int)
	cancelAt int
}

func (c *googleEarthKMLCallbackContext) Err() error {
	c.calls++
	if c.onCheck != nil {
		c.onCheck(c.calls)
	}
	if c.cancelAt > 0 && c.calls >= c.cancelAt {
		return context.Canceled
	}
	return nil
}

func TestGoogleEarthKMLDetachedBeforeCallbacksAndCancellationRetry(t *testing.T) {
	points := []googleEarthKMLPoint{{Name: "original", Description: "value", Longitude: -123, Latitude: 54}}
	ctx := &googleEarthKMLCallbackContext{Context: context.Background(), onCheck: func(int) {
		points[0] = googleEarthKMLPoint{Name: "\x00", Longitude: math.NaN()}
	}}
	data, err := prepareGoogleEarthKML(ctx, "title", points)
	var observed observedGoogleEarthKML
	if err != nil || xml.Unmarshal(data, &observed) != nil || observed.Document.Marks[0].Name != "original" {
		t.Fatal("callback changed detached input", err)
	}
	points = []googleEarthKMLPoint{{Name: "name", Description: "value", Longitude: -123, Latitude: 54}, {Name: "second", Longitude: 180, Latitude: 90}}
	for _, when := range []int{1, 2, 4, 5, 6} {
		ctx := &googleEarthKMLCallbackContext{Context: context.Background(), cancelAt: when}
		if data, err := prepareGoogleEarthKML(ctx, "title", points); !errors.Is(err, context.Canceled) || data != nil {
			t.Fatal("cancelled preparation returned partial XML", when, err)
		}
	}
	if data, err := prepareGoogleEarthKML(nil, "title", points); err == nil || data != nil {
		t.Fatal("nil context silently replaced", err)
	}
	if data, err := prepareGoogleEarthKML(context.Background(), "title", points); err != nil || len(data) == 0 {
		t.Fatal("independent retry failed", err)
	}
}
