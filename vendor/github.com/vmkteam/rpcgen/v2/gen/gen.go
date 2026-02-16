package gen

import (
	"sort"
	"strings"
	"text/template"

	"github.com/vmkteam/zenrpc/v2/smd"
	"golang.org/x/text/cases"
	"golang.org/x/text/language"
)

const version = "2.5.x"

const DefinitionsPrefix = "#/definitions/"

type GeneratorData struct {
	Version      string
	Lang         string
	LocalVersion string
}

func DefaultGeneratorData() GeneratorData {
	return GeneratorData{
		Version: version,
	}
}

func (g GeneratorData) AddLangAndLocalVersion(version, lang string) GeneratorData {
	g.Lang = lang
	g.LocalVersion = version
	return g
}

// GetNamespaceNames return all namespace names from schema.
func GetNamespaceNames(schema smd.Schema) (res []string) {
	m := map[string]struct{}{}
	for name := range schema.Services {
		m[GetNamespace(name)] = struct{}{}
	}

	for name := range m {
		res = append(res, name)
	}

	sort.Strings(res)

	return res
}

// GetNamespace extract namespace from string like "namespace.method"
func GetNamespace(methodName string) string {
	return strings.Split(methodName, ".")[0]
}

// GetMethodName extract method from string like "namespace.method"
func GetMethodName(methodName string) string {
	const methodNameLen = 2

	arr := strings.Split(methodName, ".")
	if len(arr) != methodNameLen {
		return methodName
	}

	return arr[1]
}

func StringToSlice(in, separator string) []string {
	s := strings.Split(in, separator)
	var out []string
	for _, str := range s {
		if str != "" {
			out = append(out, str)
		}
	}
	return out
}

var TemplateFuncs = template.FuncMap{
	"notLast": func(index int, len int) bool {
		return index+1 != len
	},
	"title": title,
	"camelCase": func(s string) string {
		s = title(s)
		lowerFirst := strings.ToLower(s[:1])

		return lowerFirst + s[1:]
	},
}

func title(s string) string {
	if strings.EqualFold(s, "id") {
		return "ID"
	}

	c := cases.Title(language.Und, cases.NoLower)
	if strings.HasSuffix(s, "Id") {
		s = strings.TrimSuffix(s, "Id")
		s += "ID"

		return c.String(s)
	}

	return c.String(s)
}
