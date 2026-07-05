package assets

import (
	"html/template"
	"testing"
)

func TestTemplatesParse(t *testing.T) {
	t.Parallel()

	templates := Templates()
	pages := []string{"login.html", "timeline.html", "error.html"}
	for _, page := range pages {
		t.Run(page, func(t *testing.T) {
			t.Parallel()

			if _, err := template.ParseFS(templates, "layout.html", page); err != nil {
				t.Fatal(err)
			}
		})
	}
}
