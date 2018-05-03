package util

import (
	"os"
	"regexp"
	"strings"
)

var dirSplit = regexp.MustCompile("[/\\\\]")

func CleanPath(path string) string {
	return strings.Join(dirSplit.Split(path, -1), string(os.PathSeparator))
}
