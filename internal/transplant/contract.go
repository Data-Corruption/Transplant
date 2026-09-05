package transplant

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"slices"
)

var supportedFeatures = map[string][]string{
	"update":            {},
	"update.apply":      {"update"},
	"update.apply.auto": {"service", "update.apply"},
	"service":           {},
	"service.https":     {"service"},
}

func validateContract(data []byte) error {
	var contract struct {
		Version  int `json:"version"`
		Features []struct {
			Name          string   `json:"name"`
			Prerequisites []string `json:"prerequisites"`
		} `json:"features"`
	}
	d := json.NewDecoder(bytes.NewReader(data))
	d.DisallowUnknownFields()
	if err := d.Decode(&contract); err != nil {
		return fmt.Errorf("read Sprout feature contract: %w", err)
	}
	if err := d.Decode(new(any)); err != io.EOF {
		return fmt.Errorf("unexpected output after Sprout feature contract")
	}
	if contract.Version != 1 || len(contract.Features) != len(supportedFeatures) {
		return fmt.Errorf("this Sprout contract isn't supported; update transplant before trying again")
	}
	seen := make(map[string]bool)
	for _, feature := range contract.Features {
		want, ok := supportedFeatures[feature.Name]
		slices.Sort(feature.Prerequisites)
		if !ok || seen[feature.Name] || !slices.Equal(feature.Prerequisites, want) {
			return fmt.Errorf("unfamiliar Sprout feature or prerequisites for %q; update transplant before trying again", feature.Name)
		}
		seen[feature.Name] = true
	}
	return nil
}
