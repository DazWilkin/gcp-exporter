package internal

import (
	"fmt"
	"regexp"
	"strings"
	"unicode"
)

// ExtraLabel is a struct that stores labels and preserves their original order.
type ExtraLabel struct {
	OrderedKeys []string
	LabelsMap   map[string]string
}

// toSnakeCase converts a string to snake_case.
func toSnakeCase(input string) string {
	var sb strings.Builder

	for i, r := range input {
		if unicode.IsUpper(r) {
			if i > 0 && (unicode.IsLetter(r) || unicode.IsDigit(r)) {
				sb.WriteRune('_')
			}
			r = unicode.ToLower(r)
		}
		sb.WriteRune(r)
	}

	result := sb.String()
	result = regexp.MustCompile(`[^a-zA-Z0-9]+`).ReplaceAllString(result, "_")
	result = regexp.MustCompile(`_{2,}`).ReplaceAllString(result, "_")

	return strings.Trim(result, "_")
}

// ProcessExtraLabels converts a comma-separated string of labels into an ExtraLabel struct,
// ensuring the order is preserved.
func ProcessExtraLabels(labels string) ExtraLabel {
	extraLabels := ExtraLabel{
		OrderedKeys: []string{},
		LabelsMap:   make(map[string]string),
	}

	for _, label := range strings.Split(labels, ",") {
		trimmedLabel := strings.TrimSpace(label)
		if trimmedLabel != "" {
			snakeCaseLabel := fmt.Sprintf("label_%s", toSnakeCase(trimmedLabel))
			extraLabels.LabelsMap[trimmedLabel] = snakeCaseLabel
			extraLabels.OrderedKeys = append(extraLabels.OrderedKeys, trimmedLabel)
		}
	}
	return extraLabels
}

// GetLabelNamesFromExtraLabels returns the snake_case names of extra labels in correct order.
func GetLabelNamesFromExtraLabels(extraLabels ExtraLabel) []string {
	labelNames := make([]string, 0, len(extraLabels.OrderedKeys))
	for _, key := range extraLabels.OrderedKeys {
		labelNames = append(labelNames, extraLabels.LabelsMap[key])
	}
	return labelNames
}

// GetExtraLabelsValues extracts values from resource labels while maintaining the correct order.
func GetExtraLabelsValues(resourceLabels map[string]string, extraLabels ExtraLabel) []string {
	labelValues := make([]string, 0, len(extraLabels.OrderedKeys))
	for _, key := range extraLabels.OrderedKeys {
		if value, exists := resourceLabels[key]; exists {
			labelValues = append(labelValues, value)
		} else {
			labelValues = append(labelValues, "") // Add an empty string if the label is missing
		}
	}
	return labelValues
}
