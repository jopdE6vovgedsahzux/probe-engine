// +build ignore

package main

import (
	"log"
	"os"
	"text/template"
)

var codeTemplate = template.Must(template.New("code").Parse(`
// Maybe{{ .Name }} is like {{ .Name }} except that we return the real
// value only if the privacy settings allows us. Otherwise, we just
// return the default value, model.Default{{ .Name }}.
func (s *Session) Maybe{{ .Name }}() {{ .Type }} {
	out := model.Default{{ .Name }}
	if s.privacySettings.{{ .Setting }} {
		out = s.{{ .Name }}()
	}
	return out
}`))

var testTemplate = template.Must(template.New("test").Parse(`
func Test{{ .Name }}Works(t *testing.T) {
	sess := &Session{location: &model.LocationInfo{
		{{ .LocationInfoName }}: {{ .LocationInfoValue }},
	}}
	t.Run("with false setting", func(t *testing.T) {
		sess.privacySettings.{{ .Setting }} = false
		out := sess.Maybe{{ .Name }}()
		if out != model.Default{{ .Name }} {
			t.Fatal("not the value we expected")
		}
	})
	t.Run("with true setting", func(t *testing.T) {
		sess.privacySettings.{{ .Setting }} = true
		out := sess.Maybe{{ .Name }}()
		if out != {{ .ValueForTesting }} {
			t.Fatal("not the value we expected")
		}
	})
}`))

type Variable struct {
	LocationInfoName  string
	LocationInfoValue string
	Name              string
	Setting           string
	Type              string
	ValueForTesting   string
}

var Variables = []Variable{{
	LocationInfoName:  "ProbeIP",
	LocationInfoValue: `"8.8.8.8"`,
	Name:              "ProbeIP",
	Setting:           "IncludeIP",
	Type:              "string",
	ValueForTesting:   `"8.8.8.8"`,
}, {
	LocationInfoName:  "ASN",
	LocationInfoValue: `30722`,
	Name:              "ProbeASN",
	Setting:           "IncludeASN",
	Type:              "uint",
	ValueForTesting:   `30722`,
}, {
	LocationInfoName:  "ASN",
	LocationInfoValue: `30722`,
	Name:              "ProbeASNString",
	Setting:           "IncludeASN",
	Type:              "string",
	ValueForTesting:   `"AS30722"`,
}, {
	LocationInfoName:  "CountryCode",
	LocationInfoValue: `"IT"`,
	Name:              "ProbeCC",
	Setting:           "IncludeCountry",
	Type:              "string",
	ValueForTesting:   `"IT"`,
}, {
	LocationInfoName:  "NetworkName",
	LocationInfoValue: `"Vodafone Italia"`,
	Name:              "ProbeNetworkName",
	Setting:           "IncludeASN",
	Type:              "string",
	ValueForTesting:   `"Vodafone Italia"`,
}, {
	LocationInfoName:  "ResolverIP",
	LocationInfoValue: `"9.9.9.9"`,
	Name:              "ResolverIP",
	Setting:           "IncludeIP",
	Type:              "string",
	ValueForTesting:   `"9.9.9.9"`,
}, {
	LocationInfoName:  "ResolverASN",
	LocationInfoValue: `44`,
	Name:              "ResolverASN",
	Setting:           "IncludeASN",
	Type:              "uint",
	ValueForTesting:   `44`,
}, {
	LocationInfoName:  "ResolverASN",
	LocationInfoValue: `44`,
	Name:              "ResolverASNString",
	Setting:           "IncludeASN",
	Type:              "string",
	ValueForTesting:   `"AS44"`,
}, {
	LocationInfoName:  "ResolverNetworkName",
	LocationInfoValue: `"Google LLC"`,
	Name:              "ResolverNetworkName",
	Setting:           "IncludeASN",
	Type:              "string",
	ValueForTesting:   `"Google LLC"`,
}}

func writestring(fp *os.File, s string) {
	if _, err := fp.Write([]byte(s)); err != nil {
		log.Fatal(err)
	}
}

func writeSessionGeneratedGo() {
	fp, err := os.Create("session_generated.go")
	if err != nil {
		log.Fatal(err)
	}
	writestring(fp, "// Code generated by go generate; DO NOT EDIT.\n")
	writestring(fp, "\n")
	writestring(fp, "package engine\n")
	writestring(fp, "\n")
	writestring(fp, "import \"github.com/ooni/probe-engine/model\"\n")
	writestring(fp, "\n")
	writestring(fp, "//go:generate go run generate.go")
	writestring(fp, "\n")
	for _, variable := range Variables {
		err := codeTemplate.Execute(fp, variable)
		if err != nil {
			log.Fatal(err)
		}
		writestring(fp, "\n")
	}
	if err := fp.Close(); err != nil {
		log.Fatal(err)
	}
}

func writeSessionGeneratedTestGo() {
	fp, err := os.Create("session_generated_test.go")
	if err != nil {
		log.Fatal(err)
	}
	writestring(fp, "// Code generated by go generate; DO NOT EDIT.\n")
	writestring(fp, "\n")
	writestring(fp, "package engine\n")
	writestring(fp, "\n")
	writestring(fp, "import (\n")
	writestring(fp, "\t\"testing\"\n")
	writestring(fp, "\n")
	writestring(fp, "\t\"github.com/ooni/probe-engine/model\"\n")
	writestring(fp, ")\n")
	writestring(fp, "\n")
	writestring(fp, "//go:generate go run generate.go")
	writestring(fp, "\n")
	for _, variable := range Variables {
		err := testTemplate.Execute(fp, variable)
		if err != nil {
			log.Fatal(err)
		}
		writestring(fp, "\n")
	}
	if err := fp.Close(); err != nil {
		log.Fatal(err)
	}
}

func main() {
	writeSessionGeneratedGo()
	writeSessionGeneratedTestGo()
}