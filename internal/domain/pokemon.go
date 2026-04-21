package domain

import (
	"regexp"
	"strings"
)

type Pokemon struct {
	Name        string
	Description string
	Habitat     string
	IsLegendary bool
}

var whitespaceRe = regexp.MustCompile(`\s+`)

func CleanFlavorText(s string) string {
	s = strings.ReplaceAll(s, "\f", " ")
	s = strings.ReplaceAll(s, "\n", " ")
	s = strings.ReplaceAll(s, "\r", " ")
	s = strings.ReplaceAll(s, "\t", " ")
	s = whitespaceRe.ReplaceAllString(s, " ")
	return strings.TrimSpace(s)
}
