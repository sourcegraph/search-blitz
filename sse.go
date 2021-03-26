package main

import (
	"bufio"
	"io"
	"strconv"
	"strings"
)

func NewSSEScanner(r io.Reader) *SSEScanner {
	lineScanner := bufio.NewScanner(r)
	lineScanner.Buffer(nil, 64*1024*1024)
	return &SSEScanner{
		lineScanner: lineScanner,
	}
}

type SSEScanner struct {
	lineScanner *bufio.Scanner
	event       SSEEvent
	err         error
}

// Following WHATWG spec here: https://html.spec.whatwg.org/multipage/server-sent-events.html
func (s *SSEScanner) Scan() bool {
	s.event = SSEEvent{}
	var dataBuf strings.Builder

	for {
		if !s.lineScanner.Scan() {
			s.err = s.lineScanner.Err()
			return false
		}

		line := s.lineScanner.Text()
		if len(line) == 0 {
			break
		}

		if strings.HasPrefix(line, ":") {
			continue
		}

		var field, value string
		if i := strings.IndexRune(line, ':'); i >= 0 {
			field = line[:i]
			value = strings.TrimPrefix(line[i+1:], " ")
		} else {
			field = line
		}

		switch field {
		case "event":
			s.event.Type = value
		case "data":
			dataBuf.WriteString(value)
		case "id":
			s.event.ID = &value
		case "retry":
			i, err := strconv.ParseInt(value, 10, 64)
			if err == nil {
				s.event.Retry = int(i)
			}
		}
	}

	s.event.Data = dataBuf.String()
	return true
}

func (s SSEScanner) Event() SSEEvent {
	return s.event
}

func (s SSEScanner) Err() error {
	return s.err
}

type SSEEvent struct {
	ID    *string
	Type  string
	Data  string
	Retry int
}
