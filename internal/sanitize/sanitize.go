package sanitize

import (
	"bytes"
	"encoding/hex"
	"fmt"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode/utf8"
)

// Part is either a string or an int. A string is raw SQL. An int is a
// argument placeholder.
type Part any

type Query struct {
	Parts []Part
}

// utf.DecodeRune returns the utf8.RuneError for errors. But that is actually rune U+FFFD -- the unicode replacement
// character. utf8.RuneError is not an error if it is also width 3.
//
// https://github.com/jackc/pgx/issues/1380
const replacementcharacterwidth = 3

var bufPool = &sync.Pool{}

func getBuf() *bytes.Buffer {
	buf, _ := bufPool.Get().(*bytes.Buffer)
	if buf == nil {
		buf = &bytes.Buffer{}
	}

	return buf
}

func putBuf(buf *bytes.Buffer) {
	buf.Reset()
	bufPool.Put(buf)
}

var null = []byte("null")

func (q *Query) Sanitize(args ...any) (string, error) {
	argUse := make([]bool, len(args))
	buf := getBuf()
	defer putBuf(buf)

	for _, part := range q.Parts {
		switch part := part.(type) {
		case string:
			buf.WriteString(part)
		case int:
			argIdx := part - 1
			var p []byte
			if argIdx < 0 {
				return "", fmt.Errorf("first sql argument must be > 0")
			}

			if argIdx >= len(args) {
				return "", fmt.Errorf("insufficient arguments")
			}

			// Prevent SQL injection via Line Comment Creation
			// https://github.com/jackc/pgx/security/advisories/GHSA-m7wr-2xf7-cm9p
			buf.WriteByte(' ')

			arg := args[argIdx]
			switch arg := arg.(type) {
			case nil:
				p = null
			case int64:
				p = strconv.AppendInt(buf.AvailableBuffer(), arg, 10)
			case float64:
				p = strconv.AppendFloat(buf.AvailableBuffer(), arg, 'f', -1, 64)
			case bool:
				p = strconv.AppendBool(buf.AvailableBuffer(), arg)
			case []byte:
				p = quoteBytes(buf.AvailableBuffer(), arg)
			case string:
				p = quoteString(buf.AvailableBuffer(), arg)
			case time.Time:
				p = arg.Truncate(time.Microsecond).
					AppendFormat(buf.AvailableBuffer(), "'2006-01-02 15:04:05.999999999Z07:00:00'")
			default:
				return "", fmt.Errorf("invalid arg type: %T", arg)
			}
			argUse[argIdx] = true

			buf.Write(p)

			// Prevent SQL injection via Line Comment Creation
			// https://github.com/jackc/pgx/security/advisories/GHSA-m7wr-2xf7-cm9p
			buf.WriteByte(' ')
		default:
			return "", fmt.Errorf("invalid Part type: %T", part)
		}
	}

	for i, used := range argUse {
		if !used {
			return "", fmt.Errorf("unused argument: %d", i)
		}
	}
	return buf.String(), nil
}

func NewQuery(sql string) (*Query, error) {
	l := &sqlLexer{
		src:     sql,
		stateFn: rawState,
	}

	for l.stateFn != nil {
		l.stateFn = l.stateFn(l)
	}

	query := &Query{Parts: l.parts}

	return query, nil
}

func QuoteString(str string) string {
	return string(quoteString(nil, str))
}

func quoteString(dst []byte, str string) []byte {
	const quote = "'"

	n := strings.Count(str, quote)

	dst = append(dst, quote...)

	p := slices.Grow(dst[len(dst):], len(str)+2*n)

	for len(str) > 0 {
		i := strings.Index(str, quote)
		if i < 0 {
			p = append(p, str...)
			break
		}
		p = append(p, str[:i]...)
		p = append(p, "''"...)
		str = str[i+1:]
	}

	dst = append(dst, p...)

	dst = append(dst, quote...)

	return dst
}

func QuoteBytes(buf []byte) string {
	return string(quoteBytes(nil, buf))
}

func quoteBytes(dst, buf []byte) []byte {
	dst = append(dst, `'\x`...)

	n := hex.EncodedLen(len(buf))
	p := slices.Grow(dst[len(dst):], n)[:n]
	hex.Encode(p, buf)
	dst = append(dst, p...)

	dst = append(dst, `'`...)
	return dst
}

type sqlLexer struct {
	src     string
	start   int
	pos     int
	nested  int // multiline comment nesting level.
	stateFn stateFn
	parts   []Part
}

type stateFn func(*sqlLexer) stateFn

func rawState(l *sqlLexer) stateFn {
	for {
		r, width := utf8.DecodeRuneInString(l.src[l.pos:])
		l.pos += width

		switch r {
		case 'e', 'E':
			nextRune, width := utf8.DecodeRuneInString(l.src[l.pos:])
			if nextRune == '\'' {
				l.pos += width
				return escapeStringState
			}
		case '\'':
			return singleQuoteState
		case '"':
			return doubleQuoteState
		case '$':
			nextRune, _ := utf8.DecodeRuneInString(l.src[l.pos:])
			if '0' <= nextRune && nextRune <= '9' {
				if l.pos-l.start > 0 {
					l.parts = append(l.parts, l.src[l.start:l.pos-width])
				}
				l.start = l.pos
				return placeholderState
			}
		case '-':
			nextRune, width := utf8.DecodeRuneInString(l.src[l.pos:])
			if nextRune == '-' {
				l.pos += width
				return oneLineCommentState
			}
		case '/':
			nextRune, width := utf8.DecodeRuneInString(l.src[l.pos:])
			if nextRune == '*' {
				l.pos += width
				return multilineCommentState
			}
		case utf8.RuneError:
			if width != replacementcharacterwidth {
				if l.pos-l.start > 0 {
					l.parts = append(l.parts, l.src[l.start:l.pos])
					l.start = l.pos
				}
				return nil
			}
		}
	}
}

func singleQuoteState(l *sqlLexer) stateFn {
	for {
		r, width := utf8.DecodeRuneInString(l.src[l.pos:])
		l.pos += width

		switch r {
		case '\'':
			nextRune, width := utf8.DecodeRuneInString(l.src[l.pos:])
			if nextRune != '\'' {
				return rawState
			}
			l.pos += width
		case utf8.RuneError:
			if width != replacementcharacterwidth {
				if l.pos-l.start > 0 {
					l.parts = append(l.parts, l.src[l.start:l.pos])
					l.start = l.pos
				}
				return nil
			}
		}
	}
}

func doubleQuoteState(l *sqlLexer) stateFn {
	for {
		r, width := utf8.DecodeRuneInString(l.src[l.pos:])
		l.pos += width

		switch r {
		case '"':
			nextRune, width := utf8.DecodeRuneInString(l.src[l.pos:])
			if nextRune != '"' {
				return rawState
			}
			l.pos += width
		case utf8.RuneError:
			if width != replacementcharacterwidth {
				if l.pos-l.start > 0 {
					l.parts = append(l.parts, l.src[l.start:l.pos])
					l.start = l.pos
				}
				return nil
			}
		}
	}
}

// placeholderState consumes a placeholder value. The $ must have already has
// already been consumed. The first rune must be a digit.
func placeholderState(l *sqlLexer) stateFn {
	num := 0

	for {
		r, width := utf8.DecodeRuneInString(l.src[l.pos:])
		l.pos += width

		if '0' <= r && r <= '9' {
			num *= 10
			num += int(r - '0')
		} else {
			l.parts = append(l.parts, num)
			l.pos -= width
			l.start = l.pos
			return rawState
		}
	}
}

func escapeStringState(l *sqlLexer) stateFn {
	for {
		r, width := utf8.DecodeRuneInString(l.src[l.pos:])
		l.pos += width

		switch r {
		case '\\':
			_, width = utf8.DecodeRuneInString(l.src[l.pos:])
			l.pos += width
		case '\'':
			nextRune, width := utf8.DecodeRuneInString(l.src[l.pos:])
			if nextRune != '\'' {
				return rawState
			}
			l.pos += width
		case utf8.RuneError:
			if width != replacementcharacterwidth {
				if l.pos-l.start > 0 {
					l.parts = append(l.parts, l.src[l.start:l.pos])
					l.start = l.pos
				}
				return nil
			}
		}
	}
}

func oneLineCommentState(l *sqlLexer) stateFn {
	for {
		r, width := utf8.DecodeRuneInString(l.src[l.pos:])
		l.pos += width

		switch r {
		case '\\':
			_, width = utf8.DecodeRuneInString(l.src[l.pos:])
			l.pos += width
		case '\n', '\r':
			return rawState
		case utf8.RuneError:
			if width != replacementcharacterwidth {
				if l.pos-l.start > 0 {
					l.parts = append(l.parts, l.src[l.start:l.pos])
					l.start = l.pos
				}
				return nil
			}
		}
	}
}

func multilineCommentState(l *sqlLexer) stateFn {
	for {
		r, width := utf8.DecodeRuneInString(l.src[l.pos:])
		l.pos += width

		switch r {
		case '/':
			nextRune, width := utf8.DecodeRuneInString(l.src[l.pos:])
			if nextRune == '*' {
				l.pos += width
				l.nested++
			}
		case '*':
			nextRune, width := utf8.DecodeRuneInString(l.src[l.pos:])
			if nextRune != '/' {
				continue
			}

			l.pos += width
			if l.nested == 0 {
				return rawState
			}
			l.nested--

		case utf8.RuneError:
			if width != replacementcharacterwidth {
				if l.pos-l.start > 0 {
					l.parts = append(l.parts, l.src[l.start:l.pos])
					l.start = l.pos
				}
				return nil
			}
		}
	}
}

// SanitizeSQL replaces placeholder values with args. It quotes and escapes args
// as necessary. This function is only safe when standard_conforming_strings is
// on.
func SanitizeSQL(sql string, args ...any) (string, error) {
	query, err := NewQuery(sql)
	if err != nil {
		return "", err
	}
	return query.Sanitize(args...)
}
