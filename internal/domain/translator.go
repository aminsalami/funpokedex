package domain

import "context"

const (
	TranslatorYoda        = "yoda"
	TranslatorShakespeare = "shakespeare"
)

type Translator interface {
	Name() string
	Translate(ctx context.Context, text string) (string, error)
}
