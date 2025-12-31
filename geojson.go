package grid_to_isobands

import (
	"github.com/paulmach/orb"
)

type FeatureCollection struct {
	Type       string         `json:"type"`
	Features   []Feature      `json:"features"`
	Properties map[string]any `json:"properties"`
}

type Feature struct {
	Type       string         `json:"type"`
	Geometry   Polygon        `json:"geometry"`
	Properties map[string]any `json:"properties"`
}

type Polygon struct {
	Type        string     `json:"type"`
	Coordinates []orb.Ring `json:"coordinates"`
}

func NewFeatureCollection(props map[string]any) *FeatureCollection {
	return &FeatureCollection{Type: "FeatureCollection", Properties: props, Features: make([]Feature, 0)}
}

func (fc *FeatureCollection) AddRing(ring orb.Ring, props map[string]any) {
	fc.Features = append(fc.Features, Feature{
		Type: "Feature",
		Geometry: Polygon{
			Type:        "Polygon",
			Coordinates: []orb.Ring{ring},
		},
		Properties: props,
	})
}

func (fc *FeatureCollection) AddPolygon(poly orb.Polygon, props map[string]any) {
	fc.Features = append(fc.Features, Feature{
		Type: "Feature",
		Geometry: Polygon{
			Type:        "Polygon",
			Coordinates: poly,
		},
		Properties: props,
	})
}
