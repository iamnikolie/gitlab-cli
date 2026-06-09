package cmd

import (
	"bytes"
	"encoding/json"
	"os"
	"strings"

	"github.com/langgerone/gitlab-cli/internal/render"
)

// projectObject keeps only the requested fields from one decoded object.
// A field "a.b" pulls nested object a's key b and outputs it under key "a.b".
// Missing fields are skipped. Decoding uses UseNumber so integer IDs survive.
func projectObject(m map[string]any, fields []string) map[string]any {
	out := make(map[string]any, len(fields))
	for _, f := range fields {
		if base, sub, ok := strings.Cut(f, "."); ok {
			if nested, ok := m[base].(map[string]any); ok {
				if v, ok := nested[sub]; ok {
					out[f] = v
				}
			}
			continue
		}
		if v, ok := m[f]; ok {
			out[f] = v
		}
	}
	return out
}

func decodeArray(data json.RawMessage) ([]map[string]any, error) {
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.UseNumber()
	var items []map[string]any
	if err := dec.Decode(&items); err != nil {
		return nil, err
	}
	return items, nil
}

func decodeObject(data json.RawMessage) (map[string]any, error) {
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.UseNumber()
	var m map[string]any
	if err := dec.Decode(&m); err != nil {
		return nil, err
	}
	return m, nil
}

// projectList projects each element of a JSON array to fields. Returns the
// input unchanged when fields is empty or the data isn't an array of objects.
func projectList(data json.RawMessage, fields []string) json.RawMessage {
	if len(fields) == 0 {
		return data
	}
	items, err := decodeArray(data)
	if err != nil {
		return data
	}
	out := make([]map[string]any, len(items))
	for i, m := range items {
		out[i] = projectObject(m, fields)
	}
	b, err := json.Marshal(out)
	if err != nil {
		return data
	}
	return b
}

// projectOne projects a single JSON object to fields. Returns the input
// unchanged when fields is empty or the data isn't an object.
func projectOne(data json.RawMessage, fields []string) json.RawMessage {
	if len(fields) == 0 {
		return data
	}
	m, err := decodeObject(data)
	if err != nil {
		return data
	}
	b, err := json.Marshal(projectObject(m, fields))
	if err != nil {
		return data
	}
	return b
}

// activeFields returns the explicit --fields when set, else the curated default.
func activeFields(defaults []string) []string {
	if len(projectFields) > 0 {
		return projectFields
	}
	return defaults
}

// writeRaw writes raw JSON plus a trailing newline.
func writeRaw(data json.RawMessage) error {
	os.Stdout.Write(data)
	os.Stdout.Write([]byte("\n"))
	return nil
}

// renderTable projects a JSON array to the active field set and renders it as
// table (default), csv, or tsv. Callers handle the --json path themselves.
func renderTable(data json.RawMessage, defaults []string) error {
	projected := projectList(data, activeFields(defaults))
	switch outputFormat {
	case "csv":
		return render.CSV(os.Stdout, projected)
	case "tsv":
		return render.TSV(os.Stdout, projected)
	default:
		return render.List(os.Stdout, projected)
	}
}

// emitList renders a JSON array. --json/--format json emits the full raw array;
// table/csv/tsv project to the active field set (curated default or --fields).
func emitList(data json.RawMessage, defaults []string) error {
	if outputFormat == "json" || jsonOutput {
		return writeRaw(data)
	}
	return renderTable(data, defaults)
}

// emitObj renders a single JSON object. --json/--format json emits the full raw
// object; table projects to the active field set and renders KV; csv/tsv wrap
// the projected object in a one-row array.
func emitObj(data json.RawMessage, defaults []string) error {
	if outputFormat == "json" || jsonOutput {
		return writeRaw(data)
	}
	projected := projectOne(data, activeFields(defaults))
	switch outputFormat {
	case "csv":
		return render.CSV(os.Stdout, wrapArray(projected))
	case "tsv":
		return render.TSV(os.Stdout, wrapArray(projected))
	default:
		return render.KV(os.Stdout, projected)
	}
}

// wrapArray wraps a single JSON object in a one-element array for csv/tsv.
func wrapArray(obj json.RawMessage) json.RawMessage {
	b, err := json.Marshal([]json.RawMessage{obj})
	if err != nil {
		return obj
	}
	return b
}
