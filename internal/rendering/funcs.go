// Package rendering renders the human-readable Markdown mirrors of plan.json and
// execution.json, executing a text/template directly against the canonical Go model so the
// output depends only on the document's own field values.
package rendering

import (
	"bytes"
	"fmt"
	"reflect"
	"regexp"
	"strings"
	"text/template"

	"github.com/johnrichter/delivery-agent-team-tools/internal/model"
)

var collapseNL = regexp.MustCompile(`\n+`)
var tripleNL = regexp.MustCompile(`\n{3,}`)

// mdCell sanitizes a value for a Markdown table cell: escape pipes, collapse newlines, trim.
func mdCell(v any) string {
	s := fmt.Sprint(v)
	s = strings.ReplaceAll(s, "|", `\|`)
	return strings.TrimSpace(collapseNL.ReplaceAllString(s, " "))
}

// mdLine sanitizes a value for a bullet/heading: collapse newlines, trim (no pipe escaping).
func mdLine(v any) string {
	return strings.TrimSpace(collapseNL.ReplaceAllString(fmt.Sprint(v), " "))
}

// isEmptyString reports whether v is nil or a zero-length string — including a named type
// whose underlying kind is string (e.g. model.Model, model.Effort), not only the built-in type.
func isEmptyString(v any) bool {
	if v == nil {
		return true
	}
	rv := reflect.ValueOf(v)
	return rv.Kind() == reflect.String && rv.Len() == 0
}

// dflt returns def when v is nil or an empty string, the guard every optional field needs
// since Go's zero value for a string (or a named string type) is "".
func dflt(def string, v any) string {
	if isEmptyString(v) {
		return def
	}
	return fmt.Sprint(v)
}

// join renders a slice of scalars as sep-separated text, whatever the slice's element type.
func join(sep string, v any) string {
	rv := reflect.ValueOf(v)
	if rv.Kind() != reflect.Slice {
		return ""
	}
	parts := make([]string, rv.Len())
	for i := range parts {
		parts[i] = fmt.Sprint(rv.Index(i).Interface())
	}
	return strings.Join(parts, sep)
}

// tier renders a model/effort pair as "model/effort", "?" for either side left unset.
func tier(model, effort any) string {
	return dflt("?", model) + "/" + dflt("?", effort)
}

// toFloat reads a numeric template value as a float64.
func toFloat(v any) float64 {
	switch n := v.(type) {
	case float64:
		return n
	case int64:
		return float64(n)
	case int:
		return float64(n)
	default:
		return 0
	}
}

// usd renders a dollar amount at fixed precision.
func usd(prec int, v any) string {
	return fmt.Sprintf("$%.*f", prec, toFloat(v))
}

// fileSurfaces renders a task's declared file_surface entries as one comma-joined line:
// "path (kind[, required])" per entry, kind defaulted to "file" when unset.
func fileSurfaces(entries []model.FileSurfaceEntry) string {
	parts := make([]string, len(entries))
	for i, e := range entries {
		req := ""
		if e.Required {
			req = ", required"
		}
		parts[i] = mdLine(e.Path) + " (" + string(e.Kind.Resolve()) + req + ")"
	}
	return strings.Join(parts, ", ")
}

var funcs = template.FuncMap{
	"cell":         mdCell,
	"line":         mdLine,
	"dflt":         dflt,
	"join":         join,
	"tier":         tier,
	"usd":          usd,
	"fileSurfaces": fileSurfaces,
}

func mustParse(name, text string) *template.Template {
	t, err := template.New(name).Funcs(funcs).Parse(text)
	if err != nil {
		panic(err)
	}
	return t
}

// renderMirror executes tmpl against doc and collapses any run of 3+ consecutive newlines
// (from optional sections rendering as blank) down to a single blank line.
func renderMirror(tmpl *template.Template, doc any) (string, error) {
	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, doc); err != nil {
		return "", fmt.Errorf("rendering: execute template %q: %w", tmpl.Name(), err)
	}
	return tripleNL.ReplaceAllString(buf.String(), "\n\n"), nil
}
