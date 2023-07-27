// vstars2vice.go
// Matt Pharr, MIT licensed
//
// Takes a vSTARS XML file on stdin, writes a vice-format video map JSON file on stdout.
// The maps that were converted are printed to stderr.

package main

import (
	"encoding/json"
	"encoding/xml"
	"fmt"
	"golang.org/x/exp/constraints"
	"math"
	"os"
	"strconv"
)

type XMLFacilityBundle struct {
	VideoMaps []XMLVideoMaps `xml:"VideoMaps"`
}

type XMLVideoMaps struct {
	XMLName xml.Name      `xml:"VideoMaps"`
	Maps    []XMLVideoMap `xml:"VideoMap"`
}

type XMLVideoMap struct {
	LongName string       `xml:"LongName,attr"`
	Group    string       `xml:"STARSGroup,attr"`
	Elements []XMLElement `xml:"Elements>Element"`
}

type XMLElement struct {
	Type     string `xml:"http://www.w3.org/2001/XMLSchema-instance type,attr"`
	StartLon string `xml:"StartLon,attr"`
	EndLon   string `xml:"EndLon,attr"`
	StartLat string `xml:"StartLat,attr"`
	EndLat   string `xml:"EndLat,attr"`
}

type Point2LL [2]float32

func floor(v float32) float32 {
	return float32(math.Floor(float64(v)))
}

func ceil(v float32) float32 {
	return float32(math.Ceil(float64(v)))
}

func abs[V constraints.Integer | constraints.Float](x V) V {
	if x < 0 {
		return -x
	}
	return x
}

func (p Point2LL) MarshalJSON() ([]byte, error) {
	return []byte("\"" + p.DMSString() + "\""), nil
}

// DMSString returns the position in degrees minutes, seconds, e.g.
// N039.51.39.243, W075.16.29.511
func (p Point2LL) DMSString() string {
	format := func(v float32) string {
		s := fmt.Sprintf("%03d", int(v))
		v -= floor(v)
		v *= 60
		s += fmt.Sprintf(".%02d", int(v))
		v -= floor(v)
		v *= 60
		s += fmt.Sprintf(".%02d", int(v))
		v -= floor(v)
		v *= 1000
		s += fmt.Sprintf(".%03d", int(v))
		return s
	}

	var s string
	if p[1] > 0 {
		s = "N"
	} else {
		s = "S"
	}
	s += format(abs(p[1]))

	if p[0] > 0 {
		s += ",E"
	} else {
		s += ",W"
	}
	s += format(abs(p[0]))

	return s
}

func main() {
	bail := func(e error) {
		fmt.Fprintf(os.Stderr, "%v\n", e)
		os.Exit(1)
	}

	if len(os.Args) != 3 {
		fmt.Fprintf(os.Stderr, "vstars2vice [vstars-config.xml] [output.json]\n")
		os.Exit(1)
	}

	f, err := os.Open(os.Args[1])
	if err != nil {
		bail(err)
	}
	defer f.Close()

	var root XMLFacilityBundle
	d := xml.NewDecoder(f)

	if err := d.Decode(&root); err != nil {
		bail(err)
	}

	if len(root.VideoMaps) == 0 {
		// If we didn't get anything, try with the VideoMaps element at the
		// top level.
		f.Seek(0, 0) // rewind
		if err := d.Decode(&root.VideoMaps); err != nil {
			bail(err)
		}
	}

	m := make(map[string][]Point2LL)
	for _, vm := range root.VideoMaps {
		for _, videomap := range vm.Maps {
			var segs []Point2LL
			for _, el := range videomap.Elements {
				if el.Type != "Line" {
					continue
				}

				if el.StartLon == "0" && el.EndLon == "0" && el.StartLat == "0" && el.EndLat == "0" {
					continue
				}
				slat, err := strconv.ParseFloat(el.StartLat, 32)
				if err != nil {
					fmt.Fprintf(os.Stderr, "%s: %v. Skipping this segment.\n", videomap.LongName, err)
					continue
				}
				slong, err := strconv.ParseFloat(el.StartLon, 32)
				if err != nil {
					fmt.Fprintf(os.Stderr, "%s: %v. Skipping this segment.\n", videomap.LongName, err)
					continue
				}
				elat, err := strconv.ParseFloat(el.EndLat, 32)
				if err != nil {
					fmt.Fprintf(os.Stderr, "%s: %v. Skipping this segment.\n", videomap.LongName, err)
					continue
				}
				elong, err := strconv.ParseFloat(el.EndLon, 32)
				if err != nil {
					fmt.Fprintf(os.Stderr, "%s: %v. Skipping this segment.\n", videomap.LongName, err)
					continue
				}

				segs = append(segs, Point2LL{float32(slong), float32(slat)})
				segs = append(segs, Point2LL{float32(elong), float32(elat)})
			}
			if segs != nil {
				m[videomap.LongName] = segs
				fmt.Printf("Video map: \"%s\" with %d line segments\n", videomap.LongName, len(segs))
			}
		}
	}

	out, err := os.Create(os.Args[2])
	if err != nil {
		bail(err)
	}
	defer out.Close()

	enc := json.NewEncoder(out)
	enc.SetIndent("", "    ")
	if err := enc.Encode(m); err != nil {
		bail(err)
	}
}
