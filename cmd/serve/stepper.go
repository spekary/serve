package main

import (
	"bytes"

	"golang.org/x/net/html"
)

type stepper struct {
	z          *html.Tokenizer
	src        []byte
	offset     int
	tokenStart int
	tokenEnd   int
	token      html.Token
	tag        string
	text       string
	raw        []byte
}

func newStepper(src []byte) *stepper {
	s := new(stepper)
	s.src = src
	s.z = html.NewTokenizer(bytes.NewReader(s.src))
	return s
}

func (s *stepper) next() error {
	tt := s.z.Next()
	if tt == html.ErrorToken {
		// ErrorToken is returned at EOF; any other error is inside z.Err()
		if s.z.Err() != nil && s.z.Err().Error() != "EOF" {
			return s.z.Err()
		}
	}

	s.raw = s.z.Raw() // raw bytes of this token, exactly as in the input
	s.tokenStart = s.offset
	s.tokenEnd = s.offset + len(s.raw)
	s.offset = s.tokenEnd
	s.token = s.z.Token()
	return nil
}

func (s *stepper) findStartTag(tag string) error {
	for {
		err := s.next()
		if err != nil {
			return err
		}
		if (s.token.Type == html.StartTagToken || s.token.Type == html.SelfClosingTagToken) &&
			s.token.Data == tag {
			return nil
		}
	}
}

// findEndTag will scan to the end tag for the tag just read.
// will return the innerHtml of the tag after reading the end tag.
func (s *stepper) findEndTag() (innerHtml string, err error) {
	var count int = 1

	tag := s.token.Data
	start := s.tokenEnd

	for {
		err := s.next()
		if err != nil {
			return "", err
		}
		if s.token.Type == html.EndTagToken &&
			s.token.Data == tag {
			count--
			if count == 0 {
				// done
				return string(s.src[start:s.tokenStart]), nil
			}
		} else if s.token.Type == html.StartTagToken &&
			s.token.Data == tag {
			count++
		}
	}
}
