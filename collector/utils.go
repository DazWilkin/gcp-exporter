package collector

import (
	"fmt"
	"regexp"
	"strings"
	"unicode"
)

// ExtraLabel is a map where the key is the original name and the value is the snake_case version.
type ExtraLabel map[string]string

// toSnakeCase converts a string to snake_case.
func toSnakeCase(input string) string {
	var sb strings.Builder

	// Iterate over each character in the input string.
	for i, r := range input {
		// Check if the character is an uppercase letter.
		if unicode.IsUpper(r) {
			// Add "_" before uppercase letters (except at the beginning) and convert to lowercase.
			if i > 0 && (unicode.IsLetter(r) || unicode.IsDigit(r)) {
				sb.WriteRune('_')
			}
			r = unicode.ToLower(r)
		}
		sb.WriteRune(r)
	}

	// Clean non-alphanumeric characters and consecutive underscores.
	result := sb.String()
	result = regexp.MustCompile(`[^a-zA-Z0-9]+`).ReplaceAllString(result, "_")
	result = regexp.MustCompile(`_{2,}`).ReplaceAllString(result, "_")

	// Trim leading/trailing underscores.
	return strings.Trim(result, "_")
}

// ProcessExtraLabels converts a comma-separated string of labels into a map where the key is the original label
// and the value is its snake_case version.
func ProcessExtraLabels(labels string) ExtraLabel {
	extraLabels := make(ExtraLabel)
	// Use Split to divide by commas and remove extra spaces around each label.
	for _, label := range strings.Split(labels, ",") {
		// Process only non-empty labels.
		trimmedLabel := strings.TrimSpace(label)
		if trimmedLabel != "" {
			// Add the original label and its snake_case version to the map.
			extraLabels[trimmedLabel] = fmt.Sprintf("label_%s", toSnakeCase(trimmedLabel))
		}
	}
	return extraLabels
}

// GetLabelNamesFromExtraLabels returns the snake_case names of extra labels.
func GetLabelNamesFromExtraLabels(extraLabels ExtraLabel) []string {
	labelNames := make([]string, 0, len(extraLabels))
	for _, labelValue := range extraLabels {
		labelNames = append(labelNames, labelValue)
	}
	return labelNames
}

// GetExtraLabelsValues extracts values from resource labels based on extra labels.
func GetExtraLabelsValues(resourceLabels map[string]string, extraLabels ExtraLabel) []string {
	var labelValues []string
	for key := range extraLabels {
		if value, exists := resourceLabels[key]; exists {
			labelValues = append(labelValues, value)
		} else {
			labelValues = append(labelValues, "") // Add empty if label is missing
		}
	}
	return labelValues
}
