package i18n

import "golang.org/x/text/language"

type translationBuilder struct {
	domain     string
	language   language.Tag
	confidence language.Confidence
	id         string // same as msgctxt in .PO files. Disambiguates same text. Usually blank.
	message    string
	arguments  []interface{}
}

// Build returns a new translation builder.
func Build() *translationBuilder {
	return new(translationBuilder)
}

// Domain sets the domain of the builder. The Domain indicates what part of the application is responsible
// for translating strings, and allows libraries and the framework to provide their own translations.
func (b *translationBuilder) Domain(domain string) *translationBuilder {
	b.domain = domain
	return b
}

// Language sets the canonical value of the builder
func (b *translationBuilder) Language(lang language.Tag) *translationBuilder {
	b.language = lang
	return b
}

// Confidence sets the canonical value of the builder
func (b *translationBuilder) Confidence(c language.Confidence) *translationBuilder {
	b.confidence = c
	return b
}

// ID adds a context to disambiguate strings with the same message id but different meanings
func (b *translationBuilder) ID(id string) *translationBuilder {
	b.id = id
	return b
}

// Comment will add a comment to the extracted translation file, but will otherwise not change the builder
// Use this to add comments directed to the person doing the translation.
func (b *translationBuilder) Comment(_ string) *translationBuilder {
	return b
}

// Translate ends the builder and performs the translation
func (b *translationBuilder) Translate(s string) string {
	return b.translate(s)
}

func (b *translationBuilder) translate(s string) string {
	if s == "" {
		return ""
	}
	if b.domain == "" {
		b.domain = AppDomain
	}
	if b.language == language.Und {
		b.language = languages[0].Tag
	}
	b.message = s

	return translators[b.domain].Translate(b)
}

// extractBuilderFromArguments will return a new builder, but also will extract any builder-specific commands from the
// argument list, assign those to the builder, and then return what is left of the arguments after the extraction.
func extractBuilderFromArguments(args []interface{}) (b *translationBuilder) {
	b = Build()
	for _, a := range args {
		switch v := a.(type) {
		case id:
			b.ID(v.id)
		case domain:
			b.Domain(v.domain)
		case lang:
			b.Language(v.l)
		case conf:
			b.Confidence(v.c)

		case comment:
		// do thing
		default:
			// An Sprintf argument
			b.arguments = append(b.arguments, a)
		}
	}
	return
}
