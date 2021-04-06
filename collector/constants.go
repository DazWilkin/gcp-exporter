package collector

import (
	"fmt"
)

const (
	prefix = "gcp"
)

func name(c string) func(string) string {
	return func(s string) string {
		return fmt.Sprintf("%s_%s_%s", prefix, c, s)
	}
}
