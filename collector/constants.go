package collector

import (
	"fmt"
	"time"
)

const (
	prefix = "gcp"
	//TODO(dazwilkin) move this to the resource types?
	timeout = 5 * time.Second
)

func name(c string) func(string) string {
	return func(s string) string {
		return fmt.Sprintf("%s_%s_%s", prefix, c, s)
	}
}
