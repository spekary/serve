package i18n

import (
	"fmt"

	"golang.org/x/text/language"
)

// Predefined domains. Plugins can add their own domains.

const FrameworkDomain = "gr-framework"
const AppDomain = "gr-app" // aka user domain, the default domain

var translators = map[string]TranslatorI{FrameworkDomain: NonTranslator{}, AppDomain: NonTranslator{}}

// TranslatorI is the interface that translators must fulfill
type TranslatorI interface {
	// Translate returns the translation of the string contained in the translationBuilder
	Translate(b *translationBuilder) string
}

// NonTranslator is the default translator that just passes all strings through unchanged.
type NonTranslator struct {
}

func (n NonTranslator) Translate(b *translationBuilder) string {
	if b.arguments == nil {
		// Just want a passthrough. If b.message has Sprintf format commands, calling fmt.Sprintf with no arguments will err.
		return b.message
	}
	return fmt.Sprintf(b.message, b.arguments...)
}

// RegisterTranslator sets the translation service for the given domain to the given translator
func RegisterTranslator(domain string, t TranslatorI) {
	translators[domain] = t
}

// Translate translates strings into other languages.
//
// If only a message is provided, translation will be targeted to the default domain
// and default language, which will likely result in message just being passed through.
//
// Pass ID(), Domain(), Comment() and Language() parameters to modify the translation.
// Other parameters will be passed to the translator and the message will be treated like
// a fmt.Sprintf format string.
func Translate(message string, params ...interface{}) string {
	builder := extractBuilderFromArguments(params)
	return builder.Translate(message)
}

// T is a synonym for Translate. Many legacy systems use T() as a common translation command.
func T(message string, params ...interface{}) string {
	return Translate(message, params...)
}

// The following are modifiers to the Translate() function.
type id struct {
	id string
}

// ID is a parameter you can add to the Translate() function to specify a message id.
//
// Usually the message id is the same as the string being translated,
// but when multiple strings are translated that are the same but have different meaning,
// this will be required.
// This is used as the msgctxt value in PO files, and is combined with the message to make a composite id
// in golang translation files. Adding a comment is helpful in these situations.
func ID(i string) interface{} {
	return id{i}
}

type domain struct {
	domain string
}

// Domain is a parameter you can add to the Translate() function to specify a domain.
// Use this to specify an alternate domain from the default AppDomain.
// Controls, plugins, and the framework itself can use this to control which translator is used for a translation.
func Domain(d string) interface{} {
	return domain{d}
}

type comment struct {
	comment string
}

// Comment adds a comment to the translation. It is used in extracted files, but does not impact the translator.
func Comment(c string) interface{} {
	return comment{c}
}

type lang struct {
	l language.Tag
}

// Language is a parameter you can add to the Translate() function to specify a language code.
// If none is specified, the default language will be used, which will result in no translation likely.
func Language(l language.Tag) interface{} {
	return lang{l}
}

type conf struct {
	c language.Confidence
}

// Confidence is a parameter you can add to the Translate() function to specify a language confidence value.
// If none is specified, the default value will be set to zero.
// If your translators use this value, it should be set.
// This value comes from interpreting the accept-language header value.
func Confidence(c language.Confidence) interface{} {
	return conf{c}
}
