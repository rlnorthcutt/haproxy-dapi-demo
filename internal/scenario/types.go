// Package scenario defines the YAML schema for demo scenarios and helpers
// for loading them from disk.
package scenario

// Scenario is a single demo script, loaded from a YAML file.
type Scenario struct {
	ID          string        `yaml:"id"`
	Title       string        `yaml:"title"`
	Category    string        `yaml:"category"` // config | runtime | storage | client
	Reload      bool          `yaml:"reload"`
	Description string        `yaml:"description"`
	Prereqs     []string      `yaml:"prereqs"`
	Steps       []Step        `yaml:"steps"`
	Verify      []VerifyEntry `yaml:"verify"`
	Cleanup     []string      `yaml:"cleanup"`
}

// Step is one executable step within a scenario.
type Step struct {
	Title   string `yaml:"title"`
	Explain string `yaml:"explain"`
	Run     string `yaml:"run"`
}

// VerifyEntry is an assertion run after all steps complete.
type VerifyEntry struct {
	Run      string `yaml:"run"`
	Contains string `yaml:"contains"`
}
