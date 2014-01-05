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
func parseStructureArray(input string) []interface{} {
	var s scanner.Scanner

	s.Init(strings.NewReader(input))
	return parseValue(&s).([]interface{})
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

func createAnonymousStruct(key string, val interface{}) gdbStruct {
	result := make(map[string]interface{})
	result[key] = val
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
	case "=":
		return "="
	case ",":
		return parseValue(s)
	default:
		btt := []byte(tt)
		if btt[0] == '"' {
			return string(btt[1 : len(tt)-1])
		}
		return tt
	}
}

func parseArray(s *scanner.Scanner) []interface{} {
	var result []interface{}
	for val := parseValue(s); val != nil; {
		nextval := parseValue(s)
		sval, ok := nextval.(string)
		if ok {
			if equals(sval, "=") {
				// we have a [key=val,key=val,key=val] list --> create struct for each entry
				nextval := parseValue(s)
				keyval := val.(string)
				result = append(result, createAnonymousStruct(keyval, nextval))
				val = parseValue(s)
				continue
			}
		}
		result = append(result, val)
		val = nextval
	}
	return result
}
