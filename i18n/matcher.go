package i18n

import (
	"context"
	"net/http"

	"golang.org/x/text/language"
	"golang.org/x/text/language/display"
)

// ServerLanguageEntry is a description of a supported language from the server's perspective. It includes the
// information the server will need to describe the language to the browser and to do the translation.
//
// Set the dictionary from the list of available dictionaries. This will cause only that dictionary to
// be compiled in, and the linker will then remove the others.
//
// Example:
//
//	SetSupportedLanguages (
//	  {Tag: language.AmericanEnglish, Dict: display.English},
//	  {Tag: language.German, Dict: display.German},
//	)
type ServerLanguageEntry struct {
	// Tag is the language tag for the language
	Tag language.Tag
	// Dict is the corresponding language dictionary, helping us to describe the language to users
	Dict *display.Dictionary
	// LangString is the string to display in the lang attribute of the html tag. Leave it blank to get the default from the Tag.
	LangString string
}

var languages = []ServerLanguageEntry{
	{Tag: language.AmericanEnglish, Dict: display.English, LangString: "en-US"},
}

var matcher = language.NewMatcher([]language.Tag{language.AmericanEnglish})

// SetSupportedLanguages sets up the languages that the application supports.
// You should only call this during application startup to inject your
// list of supported languages into the application.
// The first entry will be the default when no language information is
// available.
// Be sure these are canonicalized. Using the language tag constants will
// do that automatically, but if you use user text to create these language
// tags, be sure to call Canonicalize() on them.
func SetSupportedLanguages(l ...ServerLanguageEntry) {
	if len(l) < 1 {
		panic("you must have at least one language")
	}
	langTags := make([]language.Tag, len(l))
	languages = l

	for i, e := range l {
		langTags[i] = e.Tag
		if e.LangString == "" {
			e.LangString = e.Tag.String()
		}
	}

	// Set up a new matcher. Go doc says that matcher is optimized for runtime at the expense of init time.
	matcher = language.NewMatcher(langTags)
}

func defaultLanguage() language.Tag {
	return languages[0].Tag
}

// LanguageNames describes a local name and native name for a language.
type LanguageNames struct {
	LocalName  string
	NativeName string
}

// SupportedLanguageNames returns a slice of the supported languages, in both the language indicated and the native
// representation of the name of that language.
// You could use this to present a menu to the user.
func SupportedLanguageNames(t language.Tag) []LanguageNames {
	s := make([]LanguageNames, len(languages))

	for i, t := range languages {
		s[i].LocalName = t.Dict.Languages().Name(t)
		s[i].NativeName = display.Self.Name(t)
	}

	return s
}

// MatchAcceptedLanguage converts and "accept-language" header value into an index into
// the list of supported languages.
//
// Returns zero if an error occurs.
func MatchAcceptedLanguage(acceptLanguageValue string) (tag language.Tag, index int, confidence language.Confidence) {
	tags, _, err := language.ParseAcceptLanguage(acceptLanguageValue)
	if err != nil {
		t, i, c := matcher.Match(tags...)
		return t, i, c
	}
	return defaultLanguage(), 0, 0
}

type langKey struct{}

func WithLanguage(ctx context.Context, acceptLanguageValue string) context.Context {
	if acceptLanguageValue == "" {
		return ctx
	}
	_, i, _ := MatchAcceptedLanguage(acceptLanguageValue)
	return context.WithValue(ctx, langKey{}, i)
}

// LanguageFromIndex returns the language.Tag value for the index
// returned by MatchAcceptedLanguage.
//
// If the index is out of bounds, then the default language and false will be returned.
func LanguageFromIndex(i int) (language.Tag, bool) {
	if i < 0 || i >= len(languages) {
		return defaultLanguage(), false
	}
	return languages[i].Tag, true
}

// LanguageFromContext returns the language.Tag and index value for the selected
// language that was injected using WithLanguage or LanguageHandler.
//
// If no language value is detected or there is an error, then the default
// language and false will be returned.
func LanguageFromContext(ctx context.Context) (language.Tag, int, bool) {
	contextVal := ctx.Value(langKey{})
	if i, ok := contextVal.(int); !ok {
		return defaultLanguage(), 0, false
	} else {
		l, f := LanguageFromIndex(i)
		return l, i, f
	}
}

// LanguageHandler is middleware that detects the language of the request
// and inserts a context value corresponding to the closest supported
// language.
//
// Use SetSupportedLanguages to set those languages.
// Retrieve the value using LanguageFromContext.
func LanguageHandler(next http.Handler) http.Handler {
	fn := func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		ctx = WithLanguage(ctx, r.Header.Get("Accept-Language"))
		r.WithContext(ctx)
		next.ServeHTTP(w, r)
	}
	return http.HandlerFunc(fn)
}
