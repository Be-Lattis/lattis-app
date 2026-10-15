package transfer

import (
	"bufio"
	"bytes"
	"errors"
	"io"
	"strconv"
	"unicode/utf8"
)

// xmlScanner lit le XML très régulier des feuilles Excel bien plus vite
// qu'encoding/xml (qui représentait 80 % du temps d'un import) : balises,
// attributs et texte, sans validation. Les préfixes d'espace de noms sont
// ignorés, les entités standard et numériques décodées.
type xmlScanner struct {
	r     *bufio.Reader
	kind  int // xmlStart, xmlEnd, xmlText
	name  []byte
	self  bool // <balise/>
	attrs []xmlAttr
	text  []byte
	buf   []byte
}

type xmlAttr struct{ name, value []byte }

const (
	xmlStart = iota + 1
	xmlEnd
	xmlText
)

var errXML = errors.New("XML mal formé")

func newXMLScanner(r io.Reader) *xmlScanner {
	return &xmlScanner{r: bufio.NewReaderSize(r, 256<<10)}
}

func local(name []byte) []byte {
	if i := bytes.IndexByte(name, ':'); i >= 0 {
		return name[i+1:]
	}
	return name
}

// attr donne la valeur d'un attribut (nom local), nil s'il est absent.
func (s *xmlScanner) attr(name string) []byte {
	for _, a := range s.attrs {
		if string(local(a.name)) == name {
			return a.value
		}
	}
	return nil
}

// next avance au jeton suivant ; io.EOF à la fin.
func (s *xmlScanner) next() error {
	if s.kind == xmlStart && s.self {
		s.kind, s.self = xmlEnd, false
		return nil
	}
	c, err := s.r.ReadByte()
	if err != nil {
		return err
	}
	if c != '<' {
		s.r.UnreadByte()
		raw, err := s.r.ReadSlice('<')
		if err == nil {
			s.r.UnreadByte()
			raw = raw[:len(raw)-1]
		} else if err != io.EOF {
			if err != bufio.ErrBufferFull {
				return err
			}
			s.buf = append(s.buf[:0], raw...)
			for err == bufio.ErrBufferFull {
				raw, err = s.r.ReadSlice('<')
				s.buf = append(s.buf, raw...)
			}
			if err == nil {
				s.r.UnreadByte()
				s.buf = s.buf[:len(s.buf)-1]
			}
			raw = s.buf
		}
		s.kind, s.text = xmlText, unescape(raw)
		return nil
	}
	c, err = s.r.ReadByte()
	if err != nil {
		return errXML
	}
	switch c {
	case '?':
		return s.skipPast("?>")
	case '!':
		head, _ := s.r.Peek(7)
		switch {
		case bytes.HasPrefix(head, []byte("--")):
			return s.skipPast("-->")
		case bytes.HasPrefix(head, []byte("[CDATA[")):
			s.r.Discard(7)
			text, err := s.readPast("]]>")
			s.kind, s.text = xmlText, text
			return err
		}
		return s.skipPast(">")
	case '/':
		tag, err := s.readPast(">")
		if err != nil {
			return err
		}
		s.kind, s.name = xmlEnd, local(bytes.TrimSpace(tag))
		return nil
	}
	s.r.UnreadByte()
	tag, err := s.readPast(">")
	if err != nil {
		return err
	}
	s.kind, s.self, s.attrs = xmlStart, false, s.attrs[:0]
	if n := len(tag); n > 0 && tag[n-1] == '/' {
		s.self, tag = true, tag[:n-1]
	}
	i := 0
	for i < len(tag) && !space(tag[i]) {
		i++
	}
	s.name = local(tag[:i])
	for {
		for i < len(tag) && space(tag[i]) {
			i++
		}
		if i >= len(tag) {
			return nil
		}
		j := bytes.IndexByte(tag[i:], '=')
		if j < 0 {
			return errXML
		}
		name := bytes.TrimSpace(tag[i : i+j])
		i += j + 1
		for i < len(tag) && space(tag[i]) {
			i++
		}
		if i >= len(tag) || tag[i] != '"' && tag[i] != '\'' {
			return errXML
		}
		q := tag[i]
		k := bytes.IndexByte(tag[i+1:], q)
		if k < 0 {
			return errXML
		}
		s.attrs = append(s.attrs, xmlAttr{name, unescape(tag[i+1 : i+1+k])})
		i += k + 2
	}
}

func space(c byte) bool { return c == ' ' || c == '\t' || c == '\n' || c == '\r' }

// readPast lit jusqu'au délimiteur (exclu, consommé). Le résultat est
// valable jusqu'au prochain appel.
func (s *xmlScanner) readPast(end string) ([]byte, error) {
	s.buf = s.buf[:0]
	last := end[len(end)-1]
	for {
		chunk, err := s.r.ReadSlice(last)
		s.buf = append(s.buf, chunk...)
		switch {
		case err == bufio.ErrBufferFull:
			continue
		case err != nil:
			return nil, errXML
		}
		if bytes.HasSuffix(s.buf, []byte(end)) {
			return s.buf[:len(s.buf)-len(end)], nil
		}
	}
}

func (s *xmlScanner) skipPast(end string) error {
	if _, err := s.readPast(end); err != nil {
		return err
	}
	return s.next()
}

var entities = map[string]rune{"amp": '&', "lt": '<', "gt": '>', "quot": '"', "apos": '\''}

// unescape décode les entités ; sans &, le texte est rendu tel quel.
func unescape(b []byte) []byte {
	if bytes.IndexByte(b, '&') < 0 {
		return b
	}
	out := make([]byte, 0, len(b))
	for len(b) > 0 {
		i := bytes.IndexByte(b, '&')
		if i < 0 {
			return append(out, b...)
		}
		out = append(out, b[:i]...)
		b = b[i:]
		j := bytes.IndexByte(b, ';')
		if j < 0 {
			return append(out, b...)
		}
		ent := string(b[1:j])
		r, ok := entities[ent]
		if !ok && len(ent) > 1 && ent[0] == '#' {
			var n uint64
			var err error
			if ent[1] == 'x' || ent[1] == 'X' {
				n, err = strconv.ParseUint(ent[2:], 16, 32)
			} else {
				n, err = strconv.ParseUint(ent[1:], 10, 32)
			}
			r, ok = rune(n), err == nil
		}
		if !ok {
			out = append(out, '&')
			b = b[1:]
			continue
		}
		out = utf8.AppendRune(out, r)
		b = b[j+1:]
	}
	return out
}
