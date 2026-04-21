package domain

import "context"

type Translator interface {
	Translate(ctx context.Context, text string) (string, error)
}

const (
	TranslatorYoda        = "yoda"
	TranslatorShakespeare = "shakespeare"
)

func ChooseTranslatorName(habitat string, isLegendary bool) string {
	if habitat == "cave" || isLegendary {
		return TranslatorYoda
	}
	return TranslatorShakespeare
}
