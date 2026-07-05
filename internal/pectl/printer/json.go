package printer

import (
	"encoding/json"
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

// PrintJSON prints pretty-formatted JSON to stdout.
func PrintJSON(data interface{}) error {
	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(data); err != nil {
		return fmt.Errorf("failed to print JSON: %w", err)
	}
	return nil
}

// PrintYAML prints formatted YAML to stdout.
func PrintYAML(data interface{}) error {
	encoder := yaml.NewEncoder(os.Stdout)
	encoder.SetIndent(2)
	defer encoder.Close()
	if err := encoder.Encode(data); err != nil {
		return fmt.Errorf("failed to print YAML: %w", err)
	}
	return nil
}
