package parser

import (
	"bufio"
	"bytes"
	"fmt"
	"regexp"
	"strings"
)

type (
	Parser struct {
		Message string
		Mode    string
	}
)

var modes = []string{"summary", "simple", "full"}

func Parse(p *Parser) (string, error) {
	var b bytes.Buffer

	if !contains(modes, p.Mode) {
		return "", fmt.Errorf("Mode is invalid, required one of [%s]", strings.Join(modes, ","))
	}

	r := strings.NewReader(p.Message)
	scanner := bufio.NewScanner(r)

	for scanner.Scan() {
		rHash, _ := regexp.Compile("^#")
		rSum, _ := regexp.Compile("^Plan:")
		rSymbol, _ := regexp.Compile("^\\s{2}[\\+\\-\\~#]")
		rNothing, _ := regexp.Compile("This plan does nothing.")

		s := strings.TrimLeft(scanner.Text(), " ")
		switch p.Mode {
		case "full":
			if rSymbol.MatchString(scanner.Text()) {
				_, _ = b.WriteString(fmt.Sprintf("%s\n", s))
			} else {
				_, _ = b.WriteString(fmt.Sprintf("%s\n", scanner.Text()))
			}
			break
		case "simple":
			if rHash.MatchString(s) {
				_, _ = b.WriteString(fmt.Sprintf("%s\n", s))
			}
			if rSum.MatchString(s) {
				_, _ = b.WriteString(fmt.Sprintf("\n%s\n", s))
			}
			if rNothing.MatchString(s) {
				_, _ = b.WriteString(fmt.Sprintf("%s\n", scanner.Text()))
			}
			break
		default:
			if rSum.MatchString(s) {
				_, _ = b.WriteString(fmt.Sprintf("%s\n", s))
			}
			if rNothing.MatchString(s) {
				_, _ = b.WriteString(fmt.Sprintf("%s\n", scanner.Text()))
			}
		}

		continue
	}

	if err := scanner.Err(); err != nil {
		return "", err
	}

	return b.String(), nil
}

func contains(arr []string, str string) bool {
	for _, a := range arr {
		if a == str {
			return true
		}
	}
	return false
}
