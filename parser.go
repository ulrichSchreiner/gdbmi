package gdbmi

import (
	"bytes"
	"strings"
	"text/scanner"
)

type gdbStruct map[string]interface{}

var assignment []byte = []byte("=")

func parseStructure(input string) gdbStruct {
	var s scanner.Scanner

	s.Init(strings.NewReader(input))
	return parseValue(&s).(gdbStruct)
}

func parseStruct(s *scanner.Scanner) gdbStruct {
	result := make(map[string]interface{})
struct_loop:
	for {
		s.Scan()
		key := s.TokenText()
		s.Scan()
		assign := s.TokenText()
		for !bytes.Equal([]byte(assign), assignment) {
			key = key + assign
			s.Scan()
			assign = s.TokenText()
		}
		val := parseValue(s)
		result[key] = val
		s.Scan()
		delim := s.TokenText()
		switch delim {
		case "}":
			break struct_loop
		}
	}
	return result
}

func parseValue(s *scanner.Scanner) interface{} {
	s.Scan()
	tt := s.TokenText()
	switch tt {
	case "{":
		return parseStruct(s)
	case "[":
		return parseArray(s)
	case "]":
		return nil
	default:
		return string([]byte(tt)[1 : len(tt)-1])
	}
}

func parseArray(s *scanner.Scanner) []interface{} {
	var result []interface{}
	for val := parseValue(s); val != nil; val = parseValue(s) {
		result = append(result, val)
	}
	return result
}
