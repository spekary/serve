package main

import (
	"bytes"
	"errors"
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
	controlAttribute     = "data-control"
	panelAttribute       = "data-panel"
	translateAttribute   = "data-tr"
	noTranslateAttribute = "data-notr"
)

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
		out := process(src)
		if err = os.WriteFile(newFileName, out, os.ModePerm); err != nil {
			slog.Error("failed to write file", "Filename", newFileName, "Error", err)
			continue
		}
	}
}

func process(src []byte) []byte {
	var b bytes.Buffer

	title := extractTitle(src)
	if title != "" {
		b.WriteString(renderBlockDefine("title", title))
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

	b.WriteString(renderBlockDefine("form", f))
	fa := renderAttributes(z.token.Attr)
	if fa != "" {
		b.WriteString(renderBlockDefine("form-attr", fa))
	}

	b.WriteString("{{define template}}")
	err = processBodyElements(z, &b, "form")
	if err != nil {
		slog.Error("failed to process form", "Error", err)
		return nil
	}
	b.WriteString("\n{{end template}}\n")
	b.WriteString("\n{{renderFormTemplate}}\n")
	return b.Bytes()
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
func processBodyElements(z *stepper, b *bytes.Buffer, tag string) error {
	var count = 1

Loop:
	for {
		err := z.next()
		if err != nil {
			return err
		}

		switch z.token.Type {
		case html.SelfClosingTagToken:
			err = processVoidElement(z, b)
			if err != nil {
				return err
			}

		case html.StartTagToken:
			if voidElements[z.token.Data] {
				// html5 allows these to not look like self closing tags, but still be treated as such
				err = processVoidElement(z, b)
				if err != nil {
					return err
				}
				break // break out of switch
			}
			if _, ok := getAttributeValue(controlAttribute, z.token.Attr); ok {
				// substitute tag for a control
				if i, ok2 := getAttributeValue("id", z.token.Attr); !ok2 {
					return errors.New("A " + controlAttribute + " control must have an id")
				} else {
					b.WriteString(renderDrawControl(i, z.token.Attr)) // must go before findEndTag
					_, err = z.findEndTag()
					if err != nil {
						return err
					}
				}
			} else if panelObj, ok := getAttributeValue(panelAttribute, z.token.Attr); ok {
				// substitute tag for a control that has a draw function
				if i, ok2 := getAttributeValue("id", z.token.Attr); !ok2 {
					return errors.New("A " + panelAttribute + " control must have an id")
				} else {
					if z.token.Data != "div" {
						// Write code to set the tag of the panel
						b.WriteString("{{setTag ")
						b.WriteString(i)
						b.WriteString(",")
						b.WriteString(z.token.Data)
						b.WriteString("}}")
					}
					b.WriteString(renderDrawControl(i, z.token.Attr))
					err = processPanelHtml(z, panelObj, z.token.Data)
					if err != nil {
						return err
					}
				}
			} else if _, ok := getAttributeValue(translateAttribute, z.token.Attr); ok || translatableTags[z.token.Data] {
				// translate innerHtml
				b.WriteString(renderTag(z))
				if z.token.Data == tag {
					count++
				}
				err = processBodyElements(z, b, z.token.Data)
				if err != nil {
					return err
				}
				b.Write(z.raw) // write the end tag
			} else {
				// process the content of the tag without trying to translate anything
				// A <code> tag is an example of something needing this treatment.
				b.WriteString(renderTag(z))
				var ih string
				if ih, err = z.findEndTag(); err != nil {
					return err
				} else {
					b.WriteString(ih) // write the inner html
				}
				b.Write(z.raw) // write the end tag
			}

		case html.EndTagToken:
			if z.token.Data == tag {
				count--
			}
			if count == 0 {
				// done
				return nil
			}
			b.Write(z.raw) // write the end tag

		case html.TextToken:
			s, m, e := splitWhitespace(z.token.Data)
			b.WriteString(s)
			if m != "" {
				b.WriteString("{{!= ctrl.T(`")
				b.WriteString(m)
				b.WriteString("`) }}")
			}
			b.WriteString(e)

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

	b.WriteString("{{define control}}")
	b.WriteString(panelObj)
	b.WriteString("{{end control}}\n")
	b.WriteString("{{define template}}")
	err := processBodyElements(z, &b, tag)
	if err != nil {
		return err
	}

	b.WriteString("{{end template}}\n")
	b.WriteString("{{renderControlTemplate}}\n")

	filename := strings2.CamelToSnake(panelObj)
	filename = filepath.Join(outputPath, filename+extension)
	err = os.WriteFile(filename, b.Bytes(), os.ModePerm)
	return err
}

func renderAttributes(attributes []html.Attribute) string {
	var b strings.Builder

	for _, a := range attributes {
		if a.Key == controlAttribute {
			continue
		}
		if a.Key == panelAttribute {
			continue
		}
		if a.Key == formAttribute {
			continue
		}
		if a.Key == translateAttribute {
			continue
		}
		if a.Key == noTranslateAttribute {
			continue
		}
		if a.Key == "id" {
			continue
		}
		if b.Len() == 0 {
			b.WriteString(`{"`)
		} else {
			b.WriteString(`,"`)
		}
		b.WriteString(a.Key)
		b.WriteString(`":`)

		if translatableAttrs[a.Key] {
			b.WriteString(`"{{!= ctrl.T("`)
			b.WriteString(a.Val)
			b.WriteString(`") }}"`)
		} else if urlAttributes[a.Key] {
			b.WriteString(`html.MakeLocalPath("`)
			b.WriteString(a.Val)
			b.WriteString(`")`)
		} else {
			b.WriteString(`"`)
			b.WriteString(a.Val)
			b.WriteString(`"`)
		}
	}
	if b.Len() == 0 {
		return ""
	}
	b.WriteString(`}`)
	return b.String()
}

func processVoidElement(z *stepper, b *bytes.Buffer) error {
	if _, ok := getAttributeValue(controlAttribute, z.token.Attr); ok {
		if i, ok2 := getAttributeValue("id", z.token.Attr); !ok2 {
			return errors.New("A " + controlAttribute + " control must have an id")
		} else {
			b.WriteString(renderDrawControl(i, z.token.Attr))
		}
	} else {
		b.WriteString(renderTag(z))
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
			b.WriteString(`{{!= ctrl.T("`)
			b.WriteString(a.Val) // not html encoded
			b.WriteString(`") }}`)
		} else if urlAttributes[a.Key] && a.Val != "#" {
			b.WriteString(`{{= html.MakeLocalPath("`)
			b.WriteString(a.Val)
			b.WriteString(`") }}`)
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
