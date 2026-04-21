package domain

type TranslatorType string

const (
	TranslatorYoda        TranslatorType = "yoda"
	TranslatorShakespeare TranslatorType = "shakespeare"
)

func ChooseTranslator(habitat string, isLegendary bool) TranslatorType {
	if habitat == "cave" || isLegendary {
		return TranslatorYoda
	}
	return TranslatorShakespeare
}
