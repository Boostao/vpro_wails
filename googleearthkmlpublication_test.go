package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/xml"
	"errors"
	"html"
	"math"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestGoogleEarthKMLPublicationLiteralXMLHashDetachedPointsAndNeverReplace(t *testing.T) {
	directory := t.TempDir()
	path := filepath.Join(directory, "Literal output.no-inferred-extension")
	title := "<title>&\r\nliteral"
	points := []googleEarthKMLPoint{
		{Name: "<plot>", Description: "<img src='remote'> &", Longitude: -123.5, Latitude: 49.25},
		{Name: "<plot>", Description: "", Longitude: 180, Latitude: -90},
	}
	result, err := publishGoogleEarthKMLWithHooks(context.Background(), path, title, points,
		artifactPublicationHooks{observe: func(phase, _ string) error {
			if phase == "snapshot" {
				points[0].Name = "changed"
				points[0].Longitude = 190
			}
			return nil
		}})
	if err != nil || !result.Published {
		t.Fatalf("KML publication: %#v %v", result, err)
	}
	data := tableCSVPublicationRead(t, path)
	var parsed struct {
		XMLName  xml.Name
		Document struct {
			Name  string                    `xml:"name"`
			Marks []googleEarthKMLPlacemark `xml:"Placemark"`
		} `xml:"Document"`
	}
	if err := xml.Unmarshal(data, &parsed); err != nil {
		t.Fatal(err)
	}
	if parsed.XMLName != (xml.Name{Space: googleEarthKMLNamespace, Local: "kml"}) ||
		parsed.Document.Name != title || len(parsed.Document.Marks) != 2 {
		t.Fatalf("independent XML shape/title/duplicates: %#v", parsed)
	}
	if parsed.Document.Marks[0].Name != "<plot>" ||
		parsed.Document.Marks[0].Point.Coordinates != "-123.5,49.25,0" ||
		parsed.Document.Marks[0].Description != html.EscapeString("<img src='remote'> &.") ||
		parsed.Document.Marks[1].Description != "." ||
		parsed.Document.Marks[1].Point.Coordinates != "180,-90,0" ||
		bytes.Contains(data, []byte("<Style")) || bytes.Contains(data, []byte("<img")) {
		t.Fatalf("literal XML differs: %s", data)
	}
	digest := sha256.Sum256(data)
	if result.SHA256 != hex.EncodeToString(digest[:]) || !bytes.HasPrefix(data, []byte(xml.Header)) {
		t.Fatalf("exact encoded hash/header: %#v", result)
	}
	retry, err := publishGoogleEarthKML(context.Background(), path, "", nil)
	if err == nil || retry.Published || !bytes.Equal(tableCSVPublicationRead(t, path), data) {
		t.Fatalf("existing KML replaced: %#v %v", retry, err)
	}
	tableCSVPublicationOnly(t, directory, filepath.Base(path))
}

func TestGoogleEarthKMLPublicationInvalidInputsNeverLeaveArtifacts(t *testing.T) {
	for _, bad := range []struct {
		name, title string
		point       googleEarthKMLPoint
	}{
		{name: "title", title: "\xff"},
		{name: "name", point: googleEarthKMLPoint{Name: "\x00"}},
		{name: "description", point: googleEarthKMLPoint{Description: "\xff"}},
		{name: "longitude", point: googleEarthKMLPoint{Longitude: 181}},
		{name: "latitude", point: googleEarthKMLPoint{Latitude: -91}},
		{name: "nonfinite", point: googleEarthKMLPoint{Longitude: math.Inf(1)}},
	} {
		t.Run(bad.name, func(t *testing.T) {
			directory := t.TempDir()
			result, err := publishGoogleEarthKML(context.Background(), filepath.Join(directory, "output.kml"), bad.title, []googleEarthKMLPoint{bad.point})
			if err == nil || result.Published {
				t.Fatalf("invalid KML published: %#v %v", result, err)
			}
			tableCSVPublicationOnly(t, directory)
		})
	}
	directory := t.TempDir()
	result, err := publishGoogleEarthKML(nil, filepath.Join(directory, "output.kml"), "", nil)
	if err == nil || result.Published {
		t.Fatalf("nil context accepted: %#v %v", result, err)
	}
	tableCSVPublicationOnly(t, directory)
}

func TestGoogleEarthKMLPublicationCancelBeforeAndAfterCommitAndRetry(t *testing.T) {
	for _, phase := range []string{"initial", "snapshot", "staged", "prelink", "published"} {
		t.Run(phase, func(t *testing.T) {
			directory := t.TempDir()
			path := filepath.Join(directory, "output.kml")
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			if phase == "initial" {
				cancel()
			}
			result, err := publishGoogleEarthKMLWithHooks(ctx, path, "", nil, artifactPublicationHooks{
				observe: func(current, _ string) error {
					if current == phase {
						cancel()
						if current == "published" {
							return ctx.Err()
						}
					}
					return nil
				},
			})
			if !errors.Is(err, context.Canceled) || result.Published != (phase == "published") {
				t.Fatalf("cancel %s: %#v %v", phase, result, err)
			}
			if result.Published {
				before := tableCSVPublicationRead(t, path)
				retry, retryErr := publishGoogleEarthKML(context.Background(), path, "", nil)
				if !strings.Contains(err.Error(), "Google Earth KML published; do not replay publication") ||
					retryErr == nil || retry.Published || !bytes.Equal(tableCSVPublicationRead(t, path), before) {
					t.Fatalf("postcommit replay ambiguity: %#v %v", retry, retryErr)
				}
			} else {
				tableCSVPublicationOnly(t, directory)
				retry, retryErr := publishGoogleEarthKML(context.Background(), path, "", nil)
				if retryErr != nil || !retry.Published {
					t.Fatalf("cancelled retry failed: %#v %v", retry, retryErr)
				}
			}
			tableCSVPublicationOnly(t, directory, filepath.Base(path))
		})
	}
}

func TestGoogleEarthKMLPublicationLateCollisionShortWriteAndCommittedCleanupErrors(t *testing.T) {
	for _, fault := range []string{"collision", "short-write", "cleanup"} {
		t.Run(fault, func(t *testing.T) {
			directory := t.TempDir()
			path := filepath.Join(directory, "output.kml")
			hooks := artifactPublicationHooks{}
			rejected := errors.New("KML owned cleanup rejected")
			switch fault {
			case "collision":
				hooks.observe = func(phase, _ string) error {
					if phase == "prelink" {
						return os.WriteFile(path, []byte("unowned collision"), 0600)
					}
					return nil
				}
			case "short-write":
				hooks.write = func(file *os.File, data []byte) (int, error) { return file.Write(data[:len(data)-1]) }
			case "cleanup":
				hooks.remove = func(string) error { return rejected }
			}
			result, err := publishGoogleEarthKMLWithHooks(context.Background(), path, "", nil, hooks)
			if err == nil || result.Published != (fault == "cleanup") || strings.Contains(err.Error(), "table CSV") {
				t.Fatalf("KML %s result: %#v %v", fault, result, err)
			}
			if fault == "collision" && !reflect.DeepEqual(tableCSVPublicationRead(t, path), []byte("unowned collision")) {
				t.Fatal("unowned collision changed")
			}
			if fault == "short-write" {
				tableCSVPublicationOnly(t, directory)
			}
			if fault == "cleanup" && (!errors.Is(err, rejected) || !strings.Contains(err.Error(), "do not replay publication")) {
				t.Fatalf("committed cleanup error lost: %v", err)
			}
		})
	}
}
