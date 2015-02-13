package grok

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// Grok Type
type Grok struct {
	compiledPattern     *regexp.Regexp
	lastCompiledPattern string
	patterns            map[string]string
}

// New returns a Grok struct
func New() *Grok {
	o := new(Grok)
	o.patterns = map[string]string{}
	return o
}

// AddPattern add a named pattern to grok
func (g *Grok) AddPattern(name string, pattern string) {
	g.patterns[name] = pattern
}

// Compile compile a regex from the pattern
func (g *Grok) Compile(pattern string) error {
	if g.lastCompiledPattern == pattern {
		return nil
	}

	//search for %{...:...}
	r, _ := regexp.Compile(`%{(\w+:?\w+)}`)
	newPattern := pattern
	for _, values := range r.FindAllStringSubmatch(pattern, -1) {
		names := strings.Split(values[1], ":")

		customname := names[0]
		if len(names) != 1 {
			customname = names[1]
		}
		//search for replacements
		if ok := g.patterns[names[0]]; ok == "" {
			return fmt.Errorf("ERROR no pattern found for %%{%s}", names[0])
		}
		replace := fmt.Sprintf("(?P<%s>%s)", customname, g.patterns[names[0]])
		//build the new regexp
		newPattern = strings.Replace(newPattern, values[0], replace, -1)
	}

	patternCompiled, err := regexp.Compile(newPattern)
	if err != nil {
		return err
	}

	g.compiledPattern = patternCompiled
	g.lastCompiledPattern = pattern
	return nil
}

// Match returns true when text match the compileed pattern
func (g *Grok) Match(text string) (bool, error) {
	if g.compiledPattern == nil {
		return false, fmt.Errorf("No compiled pattern")
	}

	if m := g.compiledPattern.MatchString(text); !m {
		return false, nil
	}

	return true, nil
}

// Parse returns a string map with captured string based on provided pattern over the text
func (g *Grok) Parse(pattern string, text string) (map[string]string, error) {
	g.Compile(pattern)
	return g.Captures(text)
}

// Captures returns a string map with captured string on text for the compiled pattern
func (g *Grok) Captures(text string) (map[string]string, error) {
	captures := make(map[string]string)
	if g.compiledPattern == nil {
		return captures, fmt.Errorf("missing compiled regexp")
	}

	match := g.compiledPattern.FindStringSubmatch(text)
	for i, name := range g.compiledPattern.SubexpNames() {

		if len(match) > 0 {
			captures[name] = match[i]
		}

	}

	return captures, nil
}

// AddPatternsFromPath loads grok patterns from a file or files from a directory
func (g *Grok) AddPatternsFromPath(path string) error {

	if fi, err := os.Stat(path); err == nil {
		if fi.IsDir() {
			path = path + "/*"
		}
	}

	var patternDependancies = graph{}
	var fileContent = map[string]string{}

	//List all files if path folder
	files, _ := filepath.Glob(path)
	for _, file := range files {
		inFile, _ := os.Open(file)

		reader := bufio.NewReader(inFile)
		scanner := bufio.NewScanner(reader)

		for scanner.Scan() {
			l := scanner.Text()
			if len(l) > 0 { // line has text
				if l[0] != '#' { // line does not start with #
					names := strings.SplitN(l, " ", 2)
					// names[0] = key
					// names[1] = pattern
					fileContent[names[0]] = names[1]

					r, _ := regexp.Compile(`%{(\w+:?\w+)}`)
					keys := []string{}
					for _, key := range r.FindAllStringSubmatch(names[1], -1) {
						rawKey := strings.Split(key[1], ":")
						keys = append(keys, rawKey[0])
					}
					patternDependancies[names[0]] = keys
				}
			}
		}

		inFile.Close()
	}

	order, _ := sortGraph(patternDependancies)
	order = reverseList(order)

	var denormalizedPattern = map[string]string{}
	for _, key := range order {
		denormalizedPattern[key] = denormalizePattern(fileContent[key], denormalizedPattern)
		g.AddPattern(key, denormalizedPattern[key])
	}

	return nil
}

func denormalizePattern(pattern string, finalPatterns map[string]string) string {
	r, _ := regexp.Compile(`%{(\w+:?\w+)}`)
	newPattern := pattern
	for _, values := range r.FindAllStringSubmatch(pattern, -1) {
		names := strings.Split(values[1], ":")

		customname := names[0]
		if len(names) != 1 {
			customname = names[1]
		}
		//search for replacements
		replace := fmt.Sprintf("(?P<%s>%s)", customname, finalPatterns[names[0]])

		//build the new regex
		newPattern = strings.Replace(newPattern, values[0], replace, -1)

	}
	return newPattern
}
