package main

import (
	"bytes"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"unicode"

	strings2 "github.com/goradd/strings"
	"golang.org/x/net/html"
)

const (
	formAttribute        = "data-form"
	controlAttribute     = "data-control"   // if it has a value, will generate a create function and draw as a serve control. Otherwise, will just draw as a serve control.
	panelAttribute       = "data-panel"     // will generate a panel with a draw function, a create function and a draw tag.
	translateAttribute   = "data-tr"        // force translate content if tag is not normally translated
	noTranslateAttribute = "data-notr"      // force NOT translating if tag is normally translated
	saveStateAttribute   = "data-savestate" // Goes with a data-control value to set SaveState(true)
	eventsAttribute      = "data-events"    // Goes with a data-control value to specify actions based on events.
)

var customAttributes = map[string]bool{
	formAttribute:        true,
	controlAttribute:     true,
	panelAttribute:       true,
	translateAttribute:   true,
	saveStateAttribute:   true,
	eventsAttribute:      true,
	noTranslateAttribute: true,
}

// These attributes will always be translated
var translatableAttrs = map[string]bool{
	"title":            true,
	"placeholder":      true,
	"alt":              true,
	"aria-label":       true,
	"aria-description": true,
	"aria-placeholder": true,
	"aria-valuetext":   true,
	// "label": true, label in option tags is not supported in Firefox so should not be used
}

// Text nodes in these tags will always be translated
var translatableTags = map[string]bool{
	"p":          true,
	"label":      true,
	"caption":    true,
	"legend":     true,
	"figcaption": true,
	"summary":    true,
	"blockquote": true,
	"h1":         true,
	"h2":         true,
	"h3":         true,
	"h4":         true,
	"h5":         true,
	"h6":         true,
	"li":         true,
	"dt":         true,
	"dd":         true,
	"option":     true,
	"optgroup":   true,
	"button":     true,
	"span":       true,
	"div":        true,
	"a":          true,
	"em":         true,
	"strong":     true,
	"small":      true,
	"mark":       true,
	"nav":        true,
	"ol":         true,
	"ul":         true,
}

// These attributes contain urls, and so will be run through MakeLocal
var urlAttributes = map[string]bool{
	"src":    true,
	"href":   true,
	"poster": true, // for video tag
	"data":   true, // for object tag
}

// void elements in HTML (no closing tag)
// For these, the element ends at the end of its start tag.
var voidElements = map[string]bool{
	"area":   true,
	"base":   true,
	"br":     true,
	"col":    true,
	"embed":  true,
	"hr":     true,
	"img":    true,
	"input":  true,
	"link":   true,
	"meta":   true,
	"param":  true,
	"source": true,
	"track":  true,
	"wbr":    true,
}

func parse(outputPath string, files ...string) {
	for _, filename := range files {
		src, err := os.ReadFile(filename)
		if err != nil {
			slog.Error("failed to read file", "Filename", filename, "Error", err)
			continue
		}
		base := filepath.Base(filename)
		base = base + extension
		newFileName := filepath.Join(outputPath, base)
		out := processFormFile(src)
		if err = os.WriteFile(newFileName, out, os.ModePerm); err != nil {
			slog.Error("failed to write file", "Filename", newFileName, "Error", err)
			continue
		}
	}
}

func processFormFile(src []byte) []byte {
	var templateBuffer bytes.Buffer
	var creatorsBuffer bytes.Buffer
	var gettersBuffer bytes.Buffer

	title := extractTitle(src)
	if title != "" {
		templateBuffer.WriteString(renderBlockDefine("title", title))
	}
	z := newStepper(src)
	err := z.findStartTag("form")
	if err != nil {
		slog.Error("failed to find form", "Error", err)
		return nil
	}

	f, ok := getAttributeValue(formAttribute, z.token.Attr)
	if !ok || f == "" {
		slog.Error("failed to find attribute in form tag " + formAttribute)
		return nil
	}

	templateBuffer.WriteString(renderBlockDefine("form", f))
	fa := renderAttributes(z.token.Attr)
	if fa != "" {
		templateBuffer.WriteString(renderBlockDefine("form-attr", fa))
	}

	templateBuffer.WriteString("{{define template}}")
	err = processBodyElements(z, &templateBuffer, &creatorsBuffer, &gettersBuffer, "form")
	if err != nil {
		slog.Error("failed to process form", "Error", err)
		return nil
	}
	templateBuffer.WriteString("{{end template}}\n")

	if creatorsBuffer.Len() > 0 {
		templateBuffer.WriteString("\n{{define creators}}\n")
		templateBuffer.Write(creatorsBuffer.Bytes())
		templateBuffer.WriteString("\n{{end creators}}\n")
	}
	if gettersBuffer.Len() > 0 {
		templateBuffer.WriteString("{{define getters}}\n")
		templateBuffer.WriteString(gettersBuffer.String())
		templateBuffer.WriteString("\n{{end getters}}\n")
	}

	templateBuffer.WriteString("\n{{renderFormTemplate}}\n")
	return templateBuffer.Bytes()
}

func extractTitle(src []byte) string {
	z := html.NewTokenizer(bytes.NewReader(src))

	inHead := false
	inTitle := false
	var title strings.Builder

	for {
		tt := z.Next()
		if tt == html.ErrorToken {
			return html.UnescapeString(strings.TrimSpace(title.String()))
		}

		tok := z.Token()

		switch tt {
		case html.StartTagToken:
			switch tok.Data {
			case "head":
				inHead = true
			case "title":
				if inHead {
					inTitle = true
				}
			}

		case html.EndTagToken:
			switch tok.Data {
			case "head":
				inHead = false
			case "title":
				if inTitle {
					return html.UnescapeString(strings.TrimSpace(title.String()))
				}
				inTitle = false
			}

		case html.TextToken:
			if inTitle {
				title.WriteString(tok.Data)
			}
		default:
			// do nothing
		}
	}
}

// processBodyElements walks through the html, substituting got commands for known
// elements and issues.
// When this is complete, the end tag for tag will have been read, but not written.
func processBodyElements(z *stepper,
	templateBuffer *bytes.Buffer,
	creatorsBuffer *bytes.Buffer,
	gettersBuffer *bytes.Buffer,
	tag string) error {

	var count = 1

Loop:
	for {
		err := z.next()
		if err != nil {
			return err
		}

		switch z.token.Type {
		case html.SelfClosingTagToken:
			err = processVoidElement(z, templateBuffer, creatorsBuffer, gettersBuffer)
			if err != nil {
				return err
			}

		case html.StartTagToken:
			if voidElements[z.token.Data] {
				// html5 allows these to not look like self-closing tags, but still be treated as such
				err = processVoidElement(z, templateBuffer, creatorsBuffer, gettersBuffer)
				if err != nil {
					return err
				}
				break // break out of switch
			}

			attr := z.token.Attr // preserve attributes of start tag

			panelObj, hasPanel := getAttributeValue(panelAttribute, attr)
			_, hasNoTranslate := getAttributeValue(noTranslateAttribute, attr)
			_, hasTranslate := getAttributeValue(translateAttribute, attr)
			controlType, hasControlAttribute := getAttributeValue(controlAttribute, attr)
			id, hasID := getAttributeValue("id", attr)

			if hasPanel {
				if hasTranslate || hasNoTranslate {
					return errors.New("A " + panelAttribute + " control cannot be translated since its content is extracted into a generated panel drawing function")
				}

				// Use the content as a draw function for a control that we create as a panel.
				if !hasID {
					return errors.New("A " + panelAttribute + " control must have an id")
				} else {
					// Need to clone
					if z.token.Data != "div" {
						// Write code to set the tag of the panel
						templateBuffer.WriteString("{{setTag ")
						templateBuffer.WriteString(id)
						templateBuffer.WriteString(",")
						templateBuffer.WriteString(z.token.Data)
						templateBuffer.WriteString("}}")
					}
					err = processPanelHtml(z, panelObj, z.token.Data)
					if err != nil {
						return err
					}
				}
				if !hasControlAttribute {
					hasControlAttribute = true
					controlType = panelObj
				}
			}

			if hasControlAttribute {
				if hasTranslate || hasNoTranslate {
					return errors.New("A " + controlAttribute + " control cannot be translated since its content is controlled elsewhere")
				}
				// substitute tag for a control
				if !hasID {
					return errors.New("A " + controlAttribute + " control must have an id")
				}
				if controlType != "" {
					// control attribute specifies a control type. Put it in the control creators.
					templateBuffer.WriteString(renderDrawControl(id, nil)) // move attribute setting to control creation
					err = saveControlCreator(creatorsBuffer, controlType, id, attr)
					if err != nil {
						return err
					}
					saveControlGetter(gettersBuffer, controlType, id)
				} else {
					templateBuffer.WriteString(renderDrawControl(id, attr)) // must go before findEndTag
				}
				if !hasPanel {
					// Read the end tag
					if _, err = z.findEndTag(); err != nil {
						return err
					}
				}
			} else if hasTranslate ||
				(translatableTags[z.token.Data] && !hasNoTranslate) {
				// translate innerHtml
				templateBuffer.WriteString(renderTag(z))
				err = processBodyElements(z, templateBuffer, creatorsBuffer, gettersBuffer, z.token.Data) // recurse
				if err != nil {
					return err
				}
				templateBuffer.Write(z.raw) // write the end tag
			} else {
				// process the content of the tag without trying to process or translate anything
				// A <code> tag is an example of something needing this treatment.
				templateBuffer.WriteString(renderTag(z))
				var ih string
				if ih, err = z.findEndTag(); err != nil {
					return err
				} else {
					templateBuffer.WriteString(ih) // write the inner html
				}
				templateBuffer.Write(z.raw) // write the end tag
			}

		case html.EndTagToken:
			if z.token.Data == tag {
				count--
			}
			if count == 0 {
				// done
				return nil
			}
			templateBuffer.Write(z.raw) // write the end tag

		case html.TextToken:
			s, m, e := splitWhitespace(z.token.Data)
			templateBuffer.WriteString(s)
			if m != "" {
				templateBuffer.WriteString("{{tr `")
				templateBuffer.WriteString(m)
				templateBuffer.WriteString("` }}")
			}
			templateBuffer.WriteString(e)

		default:
			// eof
			break Loop
		}
	}

	return nil
}

func renderBlockDefine(blockName string, block string) string {
	var b strings.Builder
	b.WriteString("{{define ")
	b.WriteString(blockName)
	b.WriteString("}}")
	b.WriteString(block)
	b.WriteString("{{end ")
	b.WriteString(blockName)
	b.WriteString("}}\n")
	return b.String()
}

func renderDrawControl(id string, attrs []html.Attribute) string {
	var b strings.Builder

	b.WriteString("{{draw ")
	b.WriteString(id)

	attrMap := renderAttributes(attrs)
	if attrMap != "" {
		b.WriteString(", `")
		b.WriteString(attrMap)
		b.WriteString("`")
	}

	b.WriteString(" }}")

	return b.String()
}

func getAttributeValue(key string, attrs []html.Attribute) (string, bool) {
	for _, a := range attrs {
		if a.Key == key {
			return a.Val, true
		}
	}
	return "", false

}

// Convert the inner html of a panel to a panel drawing template.
func processPanelHtml(z *stepper, panelObj string, tag string) error {
	var b bytes.Buffer
	var templateBuffer bytes.Buffer
	var creatorsBuffer bytes.Buffer
	var gettersBuffer bytes.Buffer

	err := processBodyElements(z, &templateBuffer, &creatorsBuffer, &gettersBuffer, tag)
	if err != nil {
		return err
	}
	s := strings2.TrimShiftLines(templateBuffer.String())

	b.WriteString("{{define control}}")
	b.WriteString(panelObj)
	b.WriteString("{{end control}}\n")
	b.WriteString("{{define template}}")
	b.WriteString(s)
	b.WriteString("{{end template}}\n")
	if creatorsBuffer.Len() > 0 {
		b.WriteString("{{define creators}}")
		b.WriteString(creatorsBuffer.String())
		b.WriteString("{{end creators}}\n")
	}
	if gettersBuffer.Len() > 0 {
		b.WriteString("{{define getters}}\n")
		b.WriteString(gettersBuffer.String())
		b.WriteString("\n{{end getters}}\n")
	}

	b.WriteString("{{renderPanel}}\n")

	filename := strings2.CamelToSnake(panelObj)
	filename = filepath.Join(outputPath, filename+extension)
	err = os.WriteFile(filename, b.Bytes(), os.ModePerm)
	return err
}

func renderAttributes(attributes []html.Attribute) string {
	var b strings.Builder

	for _, a := range attributes {
		if customAttributes[a.Key] {
			continue
		}
		if a.Key == "id" {
			continue
		}
		if b.Len() == 0 {
			b.WriteString(`"`)
		} else {
			b.WriteString(`,"`)
		}
		b.WriteString(a.Key)
		b.WriteString(`":`)

		if translatableAttrs[a.Key] {
			// Got does not support a tag in a tag param
			b.WriteString("ctrl.T(`")
			b.WriteString(a.Val)
			b.WriteString("`)")
		} else if urlAttributes[a.Key] {
			b.WriteString("http.MakeLocalPath(`")
			b.WriteString(a.Val)
			b.WriteString("`)")
		} else {
			b.WriteRune('"')
			b.WriteString(a.Val)
			b.WriteRune('"')
		}
	}
	if b.Len() == 0 {
		return ""
	}
	return b.String()
}

func processVoidElement(z *stepper,
	templateBuffer *bytes.Buffer,
	creatorsBuffer *bytes.Buffer,
	gettersBuffer *bytes.Buffer,
) error {
	if v, ok := getAttributeValue(controlAttribute, z.token.Attr); ok {
		if i, ok2 := getAttributeValue("id", z.token.Attr); !ok2 {
			return errors.New("A " + controlAttribute + " control must have an id")
		} else if v != "" {
			// control attribute specifies a control type
			templateBuffer.WriteString(renderDrawControl(i, nil)) // move attribute setting to control creation
			if err := saveControlCreator(creatorsBuffer, v, i, z.token.Attr); err != nil {
				return err
			}
			saveControlGetter(gettersBuffer, v, i)
		} else {
			templateBuffer.WriteString(renderDrawControl(i, z.token.Attr))
		}
	} else {
		templateBuffer.WriteString(renderTag(z))
	}
	return nil
}

func splitWhitespace(s string) (leading, middle, trailing string) {
	start := 0
	end := len(s)

	// Find first non-whitespace rune
	for start < len(s) {
		if !unicode.IsSpace(rune(s[start])) {
			break
		}
		start++
	}

	// If the string is all whitespace
	if start == len(s) {
		return s, "", ""
	}

	// Find last non-whitespace rune
	for end > start {
		if !unicode.IsSpace(rune(s[end-1])) {
			break
		}
		end--
	}

	leading = s[:start]
	middle = s[start:end]
	trailing = s[end:]
	return
}

func renderTag(z *stepper) string {
	var b strings.Builder

	b.WriteString("<")
	b.WriteString(z.token.Data)

	for _, a := range z.token.Attr {
		b.WriteString(" ")
		b.WriteString(a.Key)
		if a.Val == "" {
			continue
		}
		b.WriteString(`="`)
		if translatableAttrs[a.Key] {
			b.WriteString("{{tr `")
			b.WriteString(a.Val) // not html encoded
			b.WriteString("` }}")
		} else if urlAttributes[a.Key] && a.Val != "#" {
			b.WriteString(`{{localPath `)
			b.WriteString(a.Val)
			b.WriteString(` }}`)
		} else {
			b.WriteString(a.Val)
		}
		b.WriteString(`"`)
	}
	if z.token.Type == html.SelfClosingTagToken {
		b.WriteString("/")
	}
	b.WriteString(">")

	return b.String()
}

func saveControlCreator(creatorsBuffer *bytes.Buffer,
	controlType string,
	id string,
	attr []html.Attribute) error {

	if creatorsBuffer.Len() > 0 {
		creatorsBuffer.WriteString("\n")
	}

	// controlType should have a package name
	names := strings.Split(controlType, ".")
	if len(names) != 2 {
		return fmt.Errorf("control type must have a package name: %s", controlType)
	}

	creatorsBuffer.WriteString("{{creator ")
	creatorsBuffer.WriteString(names[0])
	creatorsBuffer.WriteString(", ")
	creatorsBuffer.WriteString(names[1])
	creatorsBuffer.WriteString(", ")
	creatorsBuffer.WriteString(id)
	creatorsBuffer.WriteString("}}")

	if _, ok := getAttributeValue(saveStateAttribute, attr); ok {
		creatorsBuffer.WriteString("{{creator-savestate}}")
	}
	if v, ok := getAttributeValue(eventsAttribute, attr); ok {
		events := strings.Split(v, ";")
		for _, event := range events {
			creatorsBuffer.WriteString("{{creator-event ")
			creatorsBuffer.WriteString(event)
			creatorsBuffer.WriteString("}}")
		}
	}
	a := renderAttributes(attr)
	if a != "" {
		creatorsBuffer.WriteString("{{creator-attr `")
		creatorsBuffer.WriteString(a)
		creatorsBuffer.WriteString("`}}")
	}
	return nil
}

func saveControlGetter(gettersBuffer *bytes.Buffer,
	controlType string,
	id string) {
	if gettersBuffer.Len() > 0 {
		gettersBuffer.WriteString("\n")
	}

	// controlType should have a package name
	names := strings.Split(controlType, ".")

	gettersBuffer.WriteString("{{getter ")
	gettersBuffer.WriteString(id)
	gettersBuffer.WriteString(", ")
	gettersBuffer.WriteString(names[0])
	gettersBuffer.WriteString(", ")
	gettersBuffer.WriteString(names[1])
	gettersBuffer.WriteString("}}")

}
