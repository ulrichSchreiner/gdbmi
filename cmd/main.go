package main

import (
	"fmt"
)

func main() {
	s := "World"
	s2 := "Hello"
	s3 := sub(s, s2)
	fmt.Printf("%s\n", s3)
}

func sub(s2, s1 string) string {
	return fmt.Sprintf("%s - %s", s1, s2)
}
