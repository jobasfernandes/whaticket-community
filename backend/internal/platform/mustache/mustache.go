package mustache

import (
	cbm "github.com/cbroglie/mustache"
)

func Render(template string, data any) (string, error) {
	if template == "" {
		return "", nil
	}
	return cbm.Render(template, data)
}

func RenderOrTemplate(template string, data any) string {
	rendered, err := Render(template, data)
	if err != nil {
		return template
	}
	return rendered
}
