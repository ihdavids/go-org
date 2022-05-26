package org

import (
	"strings"
	"text/template"
)

//    const tmpl = `Hi {{.Name}}!  {{range $i, $r := .Roles}}{{if $i}}, {{end}}{{.}}{{end}}`
//    data := map[string]interface{}{
//        "Name":     "Bob",
//        "Roles":    []string{"dbteam", "uiteam", "tester"},
//    }
//
//    s ,_:= String(tmpl).Format(data)

type AString string

func (s AString) Format(data map[string]interface{}) (out string, err error) {
	t := template.Must(template.New("").Parse(string(s)))
	builder := &strings.Builder{}
	if err = t.Execute(builder, data); err != nil {
		return
	}
	out = builder.String()
	return
}
