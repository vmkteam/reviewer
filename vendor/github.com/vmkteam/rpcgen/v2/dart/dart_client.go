package dart

import (
	"bytes"
	"fmt"
	"regexp"
	"sort"
	"strings"
	"text/template"

	"github.com/vmkteam/rpcgen/v2/gen"
	"github.com/vmkteam/zenrpc/v2/smd"
)

const (
	defaultPart = "generated_rpc_client"
	version     = "1.0.0"

	Bool   = "bool"
	Int    = "int"
	Double = "double"
	String = "String"
)

var (
	linebreakRegex = regexp.MustCompile("[\r\n]+")
)

type Namespaces []Namespace

type Namespace struct {
	Name    string
	Methods []Method
}

type Method struct {
	Name        string
	SafeName    string
	Description []string
	Parameters  []Parameter
	Returns     Parameter
	// all params and return models, used in method
	Models []Parameter
}

func (m Method) ParamsClass() string {
	return m.SafeName + "Params"
}

type Parameter struct {
	Name         string
	Description  []string
	Type         string
	BaseType     string
	ReturnType   string
	Optional     bool
	DefaultValue string
	IsArray      bool
	IsObject     bool
	Properties   []Parameter
}

func (ns Namespaces) Models() []Parameter {
	var out []Parameter
	m := make(map[string]Parameter)
	for _, namespace := range ns {
		for _, method := range namespace.Methods {
			for _, p := range method.Models {
				m[p.Type] = p
			}
		}
	}
	for _, p := range m {
		out = append(out, p)
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].Type < out[j].Type
	})
	return out
}

type TypeMapper func(in smd.JSONSchema, dartType Parameter) Parameter

type templateData struct {
	gen.GeneratorData
	Part       string
	Namespaces Namespaces
	Models     []Parameter
}

type Generator struct {
	schema smd.Schema

	settings Settings
}

type Settings struct {
	Part       string
	TypeMapper TypeMapper
}

func NewClient(schema smd.Schema, settings Settings) *Generator {
	return &Generator{schema: schema, settings: settings}
}

// Generate returns generated Dart client
func (g *Generator) Generate() ([]byte, error) {
	data := templateData{Part: defaultPart, GeneratorData: gen.DefaultGeneratorData().AddLangAndLocalVersion(version, "dart")}

	if g.settings.Part != "" {
		data.Part = g.settings.Part
	}

	for _, namespaceName := range gen.GetNamespaceNames(g.schema) {
		data.Namespaces = append(data.Namespaces, Namespace{
			Name:    namespaceName,
			Methods: g.getNamespaceMethods(g.schema, namespaceName),
		})
	}
	data.Models = data.Namespaces.Models()

	tmpl, err := template.New("dart_client").Funcs(gen.TemplateFuncs).Parse(client)
	if err != nil {
		return nil, err
	}

	// compile template
	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return nil, err
	}

	return buf.Bytes(), nil

}

// getNamespaceMethods return all namespace methods
func (g *Generator) getNamespaceMethods(schema smd.Schema, namespace string) (res []Method) {
	for name, service := range schema.Services {
		if gen.GetNamespace(name) == namespace {
			res = append(res, g.newMethod(name, service))
		}
	}

	sort.Slice(res, func(i, j int) bool {
		return res[i].Name < res[j].Name
	})

	return res
}

func (g *Generator) newMethod(methodName string, service smd.Service) Method {
	desc := linebreakRegex.ReplaceAllString(service.Description, "\n")

	method := Method{
		Name:        gen.GetMethodName(methodName),
		SafeName:    strings.ReplaceAll(gen.GetMethodName(methodName), ".", ""),
		Description: strings.Split(desc, "\n"),
		Returns:     g.prepareParameter(service.Returns),
	}
	method.Parameters = g.prepareParameters(service.Parameters)
	method.Models = g.prepareModels(service)

	return method
}

func (g *Generator) prepareModels(service smd.Service) []Parameter {
	var out []Parameter

	params := g.definitionsToParams(service)
	params = append(params, g.prepareParameter(service.Returns))
	params = append(params, g.prepareParameters(service.Parameters)...)
	// remove non-objects and empty objects
	for i, p := range params {
		if !params[i].IsObject || len(params[i].Properties) == 0 {
			continue
		}
		out = append(out, p)
	}

	return out
}

func (g *Generator) definitionsToParams(service smd.Service) []Parameter {
	var out []Parameter
	for typeName, definition := range service.Returns.Definitions {
		out = append(out, g.definitionToParam(typeName, definition))
	}
	for _, p := range service.Parameters {
		for typeName, definition := range p.Definitions {
			out = append(out, g.definitionToParam(typeName, definition))
		}
	}
	return out
}

func (g *Generator) definitionToParam(typeName string, definition smd.Definition) Parameter {
	return Parameter{
		Name:       typeName,
		Type:       typeName,
		IsObject:   definition.Type == smd.Object,
		Properties: g.propertiesToParams(definition.Properties),
	}
}

// propertiesToParams convert smd.PropertyList to []Parameter
func (g *Generator) propertiesToParams(list smd.PropertyList) []Parameter {
	var parameters []Parameter
	for _, prop := range list {
		p := Parameter{
			Name:        prop.Name,
			Optional:    prop.Optional,
			Description: gen.StringToSlice(prop.Description, "\n"),
		}

		pType := dartType(prop.Type)
		if prop.Type == smd.Object && prop.Ref != "" {
			pType = strings.TrimPrefix(prop.Ref, gen.DefinitionsPrefix)
			p.IsObject = true
		}

		if prop.Type == smd.Array {
			pType = arrayType(prop.Items)
			p.IsArray = true
		}

		p.Type = pType

		if g.settings.TypeMapper != nil {
			p = g.settings.TypeMapper(smd.JSONSchema{
				Name:        prop.Name,
				Type:        prop.Type,
				TypeName:    pType,
				Description: prop.Description,
				Optional:    prop.Optional,
			}, p)
		}
		parameters = append(parameters, p)
	}

	sort.Slice(parameters, func(i, j int) bool {
		return parameters[i].Name < parameters[j].Name
	})

	return parameters
}

func (g *Generator) prepareParameters(in []smd.JSONSchema) []Parameter {
	var out []Parameter
	for _, param := range in {
		out = append(out, g.prepareParameter(param))
	}

	return out
}

// prepareParameter create Parameter from smd.JSONSchema
func (g *Generator) prepareParameter(in smd.JSONSchema) Parameter {
	out := Parameter{
		Name:        in.Name,
		Description: gen.StringToSlice(in.Description, "\n"),
		BaseType:    dartType(in.Type),
		Optional:    in.Optional,
		Properties:  g.propertiesToParams(in.Properties),
	}

	pType := dartType(in.Type)
	if in.Type == smd.Object {
		typeName := in.TypeName
		if typeName == "" && in.Description != "" && smd.IsSMDTypeName(in.Description, in.Type) {
			typeName = in.Description
		}

		if typeName != "" {
			pType = typeName
		}
		out.IsObject = true
	}

	if in.Type == smd.Array {
		pType = arrayType(in.Items)
		out.IsArray = true
	}
	out.Type = pType
	out.ReturnType = pType

	defaultValue := ""
	if in.Default != nil {
		defaultValue = string(*in.Default)
	}
	out.DefaultValue = defaultValue

	if g.settings.TypeMapper != nil {
		out = g.settings.TypeMapper(in, out)
	}

	return out
}

// dartType convert smd types to dart types
func dartType(smdType string) string {
	switch smdType {
	case smd.String:
		return String
	case smd.Boolean:
		return Bool
	case smd.Float:
		return Double
	case smd.Integer:
		return Int
	}
	return "void"
}

func arrayType(items map[string]string) string {
	var subType string
	if scalar, ok := items["type"]; ok {
		subType = dartType(scalar)
	}
	if ref, ok := items["$ref"]; ok {
		subType = strings.TrimPrefix(ref, gen.DefinitionsPrefix)
	}
	return fmt.Sprintf("List<%s>", subType)
}
