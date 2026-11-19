package main

import (
	_ "embed"
	"text/template"
)

//go:embed reset.tmpl
var fileTemplateText string

var fileTemplate = template.Must(template.New("reset").Parse(fileTemplateText))

type generatedFunc struct {
	Receiver   string
	Name       string
	Statements []string
}

type generatedFile struct {
	Package string
	Imports string
	Funcs   []generatedFunc
}
