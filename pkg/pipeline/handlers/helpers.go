package handlers

import (
	"bytes"
	"text/template"
)

// renderTemplate executes a Go template string against a data map.
func renderTemplate(tplStr string, data map[string]any) (string, error) {
	tpl, err := template.New("").Parse(tplStr)
	if err != nil {
		return "", err
	}
	var buf bytes.Buffer
	if err := tpl.Execute(&buf, data); err != nil {
		return "", err
	}
	return buf.String(), nil
}
