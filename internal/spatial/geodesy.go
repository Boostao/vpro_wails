package spatial

import (
	"errors"
	"fmt"
	"math"
	"regexp"
	"strconv"
	"strings"

	"github.com/boostao/vpro-wails/internal/sqlutil"
)

const (
	// GRS80 / NAD83 ellipsoid parameters (EPSG:4269 / EPSG:26907..26911)
	nad83SemiMajor     = 6378137.0
	nad83InvFlattening = 298.257222101
	nad83Flattening    = 1.0 / nad83InvFlattening
	utmScaleFactor     = 0.9996
	utmFalseEasting    = 500000.0
	utmFalseNorthing   = 0.0

	MinSupportedUTMZone = 7
	MaxSupportedUTMZone = 11
)

var (
	nad83ThirdN = nad83Flattening / (2.0 - nad83Flattening)
	nad83E2     = 2.0*nad83Flattening - nad83Flattening*nad83Flattening
	nad83E      = math.Sqrt(nad83E2)
	nad83RectA  = func() float64 {
		n := nad83ThirdN
		n2 := n * n
		n4 := n2 * n2
		n6 := n4 * n2
		return (nad83SemiMajor / (1.0 + n)) * (1.0 + n2/4.0 + n4/64.0 + n6/256.0)
	}()
	// Krüger 6th-order forward coefficients (geodetic -> transverse Mercator)
	nad83Alpha = func() [6]float64 {
		n := nad83ThirdN
		n2 := n * n
		n3 := n2 * n
		n4 := n2 * n2
		n5 := n4 * n
		n6 := n3 * n3
		return [6]float64{
			0.5*n - (2.0/3.0)*n2 + (5.0/16.0)*n3 + (41.0/180.0)*n4 - (127.0/288.0)*n5 + (7891.0/37800.0)*n6,
			(13.0/48.0)*n2 - (3.0/5.0)*n3 + (557.0/1440.0)*n4 + (281.0/630.0)*n5 - (1983433.0/1935360.0)*n6,
			(61.0/240.0)*n3 - (103.0/140.0)*n4 + (15061.0/26880.0)*n5 + (167603.0/181440.0)*n6,
			(49561.0/161280.0)*n4 - (179.0/168.0)*n5 + (6601661.0/7257600.0)*n6,
			(34729.0/80640.0)*n5 - (3418889.0/1995840.0)*n6,
			(212378941.0 / 319334400.0) * n6,
		}
	}()
	// Krüger 6th-order inverse coefficients (transverse Mercator -> geodetic)
	nad83Beta = func() [6]float64 {
		n := nad83ThirdN
		n2 := n * n
		n3 := n2 * n
		n4 := n2 * n2
		n5 := n4 * n
		n6 := n3 * n3
		return [6]float64{
			0.5*n - (2.0/3.0)*n2 + (37.0/96.0)*n3 - (1.0/360.0)*n4 - (81.0/512.0)*n5 + (96199.0/604800.0)*n6,
			(1.0/48.0)*n2 + (1.0/15.0)*n3 - (437.0/1440.0)*n4 + (46.0/105.0)*n5 - (1118711.0/3870720.0)*n6,
			(17.0/480.0)*n3 - (37.0/840.0)*n4 - (209.0/4480.0)*n5 + (5569.0/90720.0)*n6,
			(4397.0/161280.0)*n4 - (11.0/504.0)*n5 - (830251.0/7257600.0)*n6,
			(4583.0/161280.0)*n5 - (108847.0/3991680.0)*n6,
			(20648693.0 / 638668800.0) * n6,
		}
	}()

	utmZonePrefixPattern = regexp.MustCompile(`^\s*0*([0-9]{1,2})\s*[A-Za-z]?\s*$`)
)

func utmCentralMeridianDeg(zone int) float64 {
	return float64((zone-1)*6 - 177)
}

func nad83UTMZoneForLon(lon float64) (int, error) {
	if math.IsNaN(lon) || math.IsInf(lon, 0) {
		return 0, errors.New("longitude must be finite")
	}
	zone := int(math.Floor((lon+180.0)/6.0)) + 1
	if lon == -114.0 {
		zone = 11
	}
	if zone < MinSupportedUTMZone || zone > MaxSupportedUTMZone {
		return 0, fmt.Errorf("longitude %.6f maps to UTM zone %d outside supported NAD83 zones %d..%d", lon, zone, MinSupportedUTMZone, MaxSupportedUTMZone)
	}
	return zone, nil
}

// ProjectNAD83LatLonToUTMZone projects NAD83 (GRS80) latitude/longitude into a specific UTM zone (7..11).
func ProjectNAD83LatLonToUTMZone(lat, lon float64, zone int) (easting, northing float64, err error) {
	if math.IsNaN(lat) || math.IsNaN(lon) || math.IsInf(lat, 0) || math.IsInf(lon, 0) {
		return 0, 0, errors.New("coordinates must be finite")
	}
	if lat < 0.0 || lat > 84.5 {
		return 0, 0, fmt.Errorf("latitude %.6f is outside NAD83 Northern Hemisphere UTM domain [0, 84.5]", lat)
	}
	if zone < MinSupportedUTMZone || zone > MaxSupportedUTMZone {
		return 0, 0, fmt.Errorf("UTM zone %d is outside supported NAD83 zones %d..%d", zone, MinSupportedUTMZone, MaxSupportedUTMZone)
	}
	lon0 := utmCentralMeridianDeg(zone)
	dLonDeg := lon - lon0
	if math.Abs(dLonDeg) > 30.0 {
		return 0, 0, fmt.Errorf("longitude %.6f is too far from UTM zone %d central meridian %.1f", lon, zone, lon0)
	}

	phi := lat * math.Pi / 180.0
	dLam := dLonDeg * math.Pi / 180.0

	tau := math.Tan(phi)
	sigma := math.Sinh(nad83E * math.Atanh(nad83E*tau/math.Hypot(1.0, tau)))
	tauPrime := tau*math.Hypot(1.0, sigma) - sigma*math.Hypot(1.0, tau)

	cosDLam := math.Cos(dLam)
	sinDLam := math.Sin(dLam)
	xiPrime := math.Atan2(tauPrime, cosDLam)
	etaPrime := math.Asinh(sinDLam / math.Hypot(tauPrime, cosDLam))

	xi := xiPrime
	eta := etaPrime
	for j := 1; j <= 6; j++ {
		jf := 2.0 * float64(j)
		a := nad83Alpha[j-1]
		xi += a * math.Sin(jf*xiPrime) * math.Cosh(jf*etaPrime)
		eta += a * math.Cos(jf*xiPrime) * math.Sinh(jf*etaPrime)
	}

	easting = utmFalseEasting + utmScaleFactor*nad83RectA*eta
	northing = utmFalseNorthing + utmScaleFactor*nad83RectA*xi
	return easting, northing, nil
}

// ProjectNAD83LatLonToUTM projects NAD83 (GRS80) latitude/longitude into its standard UTM zone (7..11).
func ProjectNAD83LatLonToUTM(lat, lon float64) (zone int, easting, northing float64, err error) {
	if math.IsNaN(lat) || math.IsNaN(lon) || math.IsInf(lat, 0) || math.IsInf(lon, 0) {
		return 0, 0, 0, errors.New("coordinates must be finite")
	}
	zone, err = nad83UTMZoneForLon(lon)
	if err != nil {
		return 0, 0, 0, err
	}
	easting, northing, err = ProjectNAD83LatLonToUTMZone(lat, lon, zone)
	if err != nil {
		return 0, 0, 0, err
	}
	return zone, easting, northing, nil
}

// ProjectNAD83UTMToLatLon projects NAD83 UTM coordinates (zones 7..11) back to NAD83 decimal degrees.
func ProjectNAD83UTMToLatLon(zone int, easting, northing float64) (lat, lon float64, err error) {
	if zone < MinSupportedUTMZone || zone > MaxSupportedUTMZone {
		return 0, 0, fmt.Errorf("UTM zone %d is outside supported NAD83 zones %d..%d", zone, MinSupportedUTMZone, MaxSupportedUTMZone)
	}
	if math.IsNaN(easting) || math.IsNaN(northing) || math.IsInf(easting, 0) || math.IsInf(northing, 0) {
		return 0, 0, errors.New("UTM easting and northing must be finite")
	}
	if easting < 100000.0 || easting > 900000.0 {
		return 0, 0, fmt.Errorf("UTM easting %.2f is outside valid range [100000, 900000]", easting)
	}
	if northing < 0.0 || northing > 10000000.0 {
		return 0, 0, fmt.Errorf("UTM northing %.2f is outside valid Northern Hemisphere range [0, 10000000]", northing)
	}

	xi := (northing - utmFalseNorthing) / (utmScaleFactor * nad83RectA)
	eta := (easting - utmFalseEasting) / (utmScaleFactor * nad83RectA)

	xiPrime := xi
	etaPrime := eta
	for j := 1; j <= 6; j++ {
		jf := 2.0 * float64(j)
		b := nad83Beta[j-1]
		xiPrime -= b * math.Sin(jf*xi) * math.Cosh(jf*eta)
		etaPrime -= b * math.Cos(jf*xi) * math.Sinh(jf*eta)
	}

	sinhEtaPrime := math.Sinh(etaPrime)
	cosXiPrime := math.Cos(xiPrime)
	sinXiPrime := math.Sin(xiPrime)
	tauPrime := sinXiPrime / math.Hypot(sinhEtaPrime, cosXiPrime)

	tau := tauPrime
	oneMinusE2 := 1.0 - nad83E2
	for iter := 0; iter < 8; iter++ {
		sigma := math.Sinh(nad83E * math.Atanh(nad83E*tau/math.Hypot(1.0, tau)))
		tauI := tau*math.Hypot(1.0, sigma) - sigma*math.Hypot(1.0, tau)
		dTau := ((tauPrime - tauI) / math.Hypot(1.0, tauI)) * ((1.0 + oneMinusE2*tau*tau) / (oneMinusE2 * math.Hypot(1.0, tau)))
		tau += dTau
		if math.Abs(dTau) < 1e-14 {
			break
		}
	}

	phi := math.Atan(tau)
	dLam := math.Atan2(sinhEtaPrime, cosXiPrime)
	lat = phi * 180.0 / math.Pi
	lon = utmCentralMeridianDeg(zone) + dLam*180.0/math.Pi
	return lat, lon, nil
}

// NormalizeBCWesternCoordinates normalizes positive Western Hemisphere longitudes (e.g. 125.4 -> -125.4)
// and checks whether coordinates fall within BC / Canadian North bounds ([46.0..84.5] N, [-144.0..-110.0] W).
func NormalizeBCWesternCoordinates(lat, lon float64) (normLat, normLon float64, flipped, inBounds bool) {
	normLat, normLon = lat, lon
	if math.IsNaN(lat) || math.IsNaN(lon) || math.IsInf(lat, 0) || math.IsInf(lon, 0) {
		return lat, lon, false, false
	}
	if normLon >= 110.0 && normLon <= 144.0 {
		normLon = -normLon
		flipped = true
	}
	inBounds = normLat >= 46.0 && normLat <= 84.5 && normLon >= -144.0 && normLon <= -110.0
	return normLat, normLon, flipped, inBounds
}

// ComputeBCGSMapSheet computes the 7-character BCGS 1:20,000 map sheet identifier (e.g. "092G025", "093H015")
// for latitudes in [48, 64) and longitudes in [-144, -112], or the NTS 1:250,000 / 1:50,000 identifier north of 64°N.
func ComputeBCGSMapSheet(lat, lon float64) string {
	normLat, normLon, _, _ := NormalizeBCWesternCoordinates(lat, lon)
	if math.IsNaN(normLat) || math.IsNaN(normLon) || math.IsInf(normLat, 0) || math.IsInf(normLon, 0) {
		return ""
	}
	if normLat < 48.0 || normLat >= 68.0 || normLon < -144.0 || normLon > -112.0 {
		return ""
	}
	absLon := -normLon
	// Primary 1:1,000,000 quadrangle (8° longitude x 4° latitude south of 68°N)
	primaryCol := int(math.Floor((absLon - 48.0) / 8.0))
	if absLon == 112.0 {
		primaryCol = 8
	}
	primaryRow := int(math.Floor((normLat - 40.0) / 4.0))
	primaryNum := primaryCol*10 + primaryRow

	latBase := 40.0 + float64(primaryRow)*4.0
	lonEast := -(48.0 + float64(primaryCol)*8.0)

	// 1:250,000 letter block (2° longitude x 1° latitude, serpentine A..P from SE corner)
	subRow := int(math.Floor(normLat - latBase))
	if subRow < 0 {
		subRow = 0
	} else if subRow > 3 {
		subRow = 3
	}
	subColFromEast := int(math.Floor((lonEast - normLon) / 2.0))
	if subColFromEast < 0 {
		subColFromEast = 0
	} else if subColFromEast > 3 {
		subColFromEast = 3
	}
	var letterIdx int
	if subRow%2 == 0 {
		letterIdx = subRow*4 + subColFromEast
	} else {
		letterIdx = subRow*4 + (3 - subColFromEast)
	}
	letter := byte('A' + letterIdx)

	// BCGS 1:20,000 sheet number (001..100: 10 rows S->N of 0.1° lat by 10 cols W->E of 0.2° lon)
	blockLatSouth := latBase + float64(subRow)*1.0
	blockLonWest := lonEast - float64(subColFromEast+1)*2.0

	row20k := int(math.Floor((normLat-blockLatSouth)*10.0 + 1e-11))
	if row20k < 0 {
		row20k = 0
	} else if row20k > 9 {
		row20k = 9
	}
	col20k := int(math.Floor((normLon-blockLonWest)*5.0+1e-11)) + 1
	if col20k < 1 {
		col20k = 1
	} else if col20k > 10 {
		col20k = 10
	}
	sheet20k := row20k*10 + col20k
	return fmt.Sprintf("%03d%c%03d", primaryNum, letter, sheet20k)
}

// EllipsoidalDistanceNAD83 computes geodesic surface distance in metres on GRS80 (Vincenty inverse with Haversine fallback).
func EllipsoidalDistanceNAD83(lat1, lon1, lat2, lon2 float64) float64 {
	if lat1 == lat2 && lon1 == lon2 {
		return 0
	}
	phi1 := lat1 * math.Pi / 180.0
	phi2 := lat2 * math.Pi / 180.0
	dLam := (lon2 - lon1) * math.Pi / 180.0

	b := nad83SemiMajor * (1.0 - nad83Flattening)
	u1 := math.Atan((1.0 - nad83Flattening) * math.Tan(phi1))
	u2 := math.Atan((1.0 - nad83Flattening) * math.Tan(phi2))
	sinU1, cosU1 := math.Sin(u1), math.Cos(u1)
	sinU2, cosU2 := math.Sin(u2), math.Cos(u2)

	lam := dLam
	for iter := 0; iter < 40; iter++ {
		sinLam, cosLam := math.Sin(lam), math.Cos(lam)
		sinSigma := math.Hypot(cosU2*sinLam, cosU1*sinU2-sinU1*cosU2*cosLam)
		if sinSigma == 0 {
			return 0
		}
		cosSigma := sinU1*sinU2 + cosU1*cosU2*cosLam
		sigma := math.Atan2(sinSigma, cosSigma)
		sinAlpha := cosU1 * cosU2 * sinLam / sinSigma
		cos2Alpha := 1.0 - sinAlpha*sinAlpha
		cos2SigmaM := 0.0
		if cos2Alpha != 0 {
			cos2SigmaM = cosSigma - 2.0*sinU1*sinU2/cos2Alpha
		}
		c := (nad83Flattening / 16.0) * cos2Alpha * (4.0 + nad83Flattening*(4.0-3.0*cos2Alpha))
		lamPrev := lam
		lam = dLam + (1.0-c)*nad83Flattening*sinAlpha*(sigma+c*sinSigma*(cos2SigmaM+c*cosSigma*(-1.0+2.0*cos2SigmaM*cos2SigmaM)))
		if math.Abs(lam-lamPrev) < 1e-12 || iter == 39 {
			uSq := cos2Alpha * (nad83SemiMajor*nad83SemiMajor - b*b) / (b * b)
			bigA := 1.0 + (uSq/16384.0)*(4096.0+uSq*(-768.0+uSq*(320.0-175.0*uSq)))
			bigB := (uSq / 1024.0) * (256.0 + uSq*(-128.0+uSq*(74.0-47.0*uSq)))
			dSigma := bigB * sinSigma * (cos2SigmaM + (bigB/4.0)*(cosSigma*(-1.0+2.0*cos2SigmaM*cos2SigmaM)-(bigB/6.0)*cos2SigmaM*(-3.0+4.0*sinSigma*sinSigma)*(-3.0+4.0*cos2SigmaM*cos2SigmaM)))
			return b * bigA * (sigma - dSigma)
		}
	}
	return 0
}

// ParseUTMZoneNumber parses a VPro UTMZone string (e.g. "10", "09", "10U") into an integer zone.
func ParseUTMZoneNumber(raw string) (int, bool) {
	m := utmZonePrefixPattern.FindStringSubmatch(raw)
	if len(m) != 2 {
		return 0, false
	}
	z, err := strconv.Atoi(m[1])
	if err != nil {
		return 0, false
	}
	return z, true
}

// NAD83UTMProjection is the Wails IPC DTO for NAD83 Lat/Lon -> UTM projection.
type NAD83UTMProjection struct {
	Zone        int     `json:"zone"`
	Easting     float64 `json:"easting"`
	Northing    float64 `json:"northing"`
	NtsMapSheet string  `json:"ntsMapSheet"`
}

// NAD83LatLonProjection is the Wails IPC DTO for NAD83 UTM -> Lat/Lon projection.
type NAD83LatLonProjection struct {
	Latitude    float64 `json:"latitude"`
	Longitude   float64 `json:"longitude"`
	NtsMapSheet string  `json:"ntsMapSheet"`
}

// PlotGeodesyInput supplies stored or draft plot coordinate fields for Auto-1 geodetic evaluation.
type PlotGeodesyInput struct {
	Latitude         *float64 `json:"latitude"`
	Longitude        *float64 `json:"longitude"`
	UTMZone          *string  `json:"utmZone"`
	UTMEasting       *float64 `json:"utmEasting"`
	UTMNorthing      *float64 `json:"utmNorthing"`
	LocationAccuracy *int     `json:"locationAccuracy"`
}

func (r *PlotGeodesyInput) UnmarshalJSON(data []byte) error {
	type plain PlotGeodesyInput
	return sqlutil.DecodeStrictJSON(data, (*plain)(r))
}

// PlotGeodesyEvaluation reports bidirectional NAD83 projection, sign normalization, and UTM-vs-DD divergence.
type PlotGeodesyEvaluation struct {
	HasLatLon            bool     `json:"hasLatLon"`
	HasUTM               bool     `json:"hasUtm"`
	NormalizedLat        *float64 `json:"normalizedLat"`
	NormalizedLon        *float64 `json:"normalizedLon"`
	LongitudeSignFlipped bool     `json:"longitudeSignFlipped"`
	ComputedUTMZone      *int     `json:"computedUtmZone"`
	ComputedUTMEasting   *float64 `json:"computedUtmEasting"`
	ComputedUTMNorthing  *float64 `json:"computedUtmNorthing"`
	UTMDerivedLat        *float64 `json:"utmDerivedLat"`
	UTMDerivedLon        *float64 `json:"utmDerivedLon"`
	DivergenceMeters     *float64 `json:"divergenceMeters"`
	ThresholdMeters      float64  `json:"thresholdMeters"`
	DivergenceFlagged    bool     `json:"divergenceFlagged"`
	BCGSMapSheet         string   `json:"bcgsMapSheet"`
	OutOfBounds          bool     `json:"outOfBounds"`
	Diagnostics          []string `json:"diagnostics"`
}

// EvaluatePlotGeodesy normalizes positive-West BC longitudes (e.g. 125.4 -> -125.4), projects NAD83
// UTM <-> DD in both directions, computes BCGS 1:20k map sheet, and flags UTM vs Lat/Lon divergence > max(LocationAccuracy, 50m).
func EvaluatePlotGeodesy(input PlotGeodesyInput) PlotGeodesyEvaluation {
	threshold := 50.0
	if input.LocationAccuracy != nil && float64(*input.LocationAccuracy) > threshold {
		threshold = float64(*input.LocationAccuracy)
	}
	eval := PlotGeodesyEvaluation{
		ThresholdMeters: threshold,
		Diagnostics:     []string{},
	}

	if input.Latitude != nil && input.Longitude != nil &&
		!math.IsNaN(*input.Latitude) && !math.IsInf(*input.Latitude, 0) &&
		!math.IsNaN(*input.Longitude) && !math.IsInf(*input.Longitude, 0) &&
		(*input.Latitude != 0 || *input.Longitude != 0) {
		normLat, normLon, flipped, inBounds := NormalizeBCWesternCoordinates(*input.Latitude, *input.Longitude)
		eval.HasLatLon = true
		eval.NormalizedLat = &normLat
		eval.NormalizedLon = &normLon
		eval.LongitudeSignFlipped = flipped
		if flipped {
			eval.Diagnostics = append(eval.Diagnostics, fmt.Sprintf("Normalized unsigned Western longitude %.5f to %.5f", *input.Longitude, normLon))
		}
		if !inBounds {
			eval.OutOfBounds = true
			eval.Diagnostics = append(eval.Diagnostics, fmt.Sprintf("Latitude/Longitude (%.5f, %.5f) is outside standard BC/Arctic bounds", normLat, normLon))
		}
		if z, e, n, err := ProjectNAD83LatLonToUTM(normLat, normLon); err == nil {
			eval.ComputedUTMZone = &z
			eval.ComputedUTMEasting = &e
			eval.ComputedUTMNorthing = &n
		}
		eval.BCGSMapSheet = ComputeBCGSMapSheet(normLat, normLon)
	}

	if input.UTMZone != nil && strings.TrimSpace(*input.UTMZone) != "" && input.UTMEasting != nil && input.UTMNorthing != nil {
		if z, ok := ParseUTMZoneNumber(*input.UTMZone); ok {
			if uLat, uLon, err := ProjectNAD83UTMToLatLon(z, *input.UTMEasting, *input.UTMNorthing); err == nil {
				eval.HasUTM = true
				eval.UTMDerivedLat = &uLat
				eval.UTMDerivedLon = &uLon
				if eval.BCGSMapSheet == "" {
					eval.BCGSMapSheet = ComputeBCGSMapSheet(uLat, uLon)
				}
			} else {
				eval.Diagnostics = append(eval.Diagnostics, fmt.Sprintf("UTM projection rejected: %v", err))
			}
		} else {
			eval.Diagnostics = append(eval.Diagnostics, fmt.Sprintf("Unrecognized UTM zone %q", *input.UTMZone))
		}
	}

	if eval.HasLatLon && eval.HasUTM && eval.NormalizedLat != nil && eval.NormalizedLon != nil && eval.UTMDerivedLat != nil && eval.UTMDerivedLon != nil {
		dist := EllipsoidalDistanceNAD83(*eval.NormalizedLat, *eval.NormalizedLon, *eval.UTMDerivedLat, *eval.UTMDerivedLon)
		eval.DivergenceMeters = &dist
		if dist > eval.ThresholdMeters {
			eval.DivergenceFlagged = true
			eval.Diagnostics = append(eval.Diagnostics, fmt.Sprintf("UTM vs Lat/Lon diverges by %.1f m (threshold %.0f m)", dist, eval.ThresholdMeters))
		}
	}

	return eval
}
