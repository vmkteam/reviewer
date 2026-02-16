package kotlin

import (
	"bytes"
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"text/template"

	"github.com/vmkteam/rpcgen/v2/gen"
	"github.com/vmkteam/zenrpc/v2/smd"
)

const (
	BasePackageAPI = "api"
	BaseClass      = "Api"
	Bool           = "Boolean"
	Int            = "Int"
	Double         = "Double"
	Float          = "Float"
	String         = "String"
	Long           = "Long"
	Timestamp      = "ZonedDateTime"
	List           = "List"
	Map            = "Map"

	DefaultBoolFalse = "false"
	DefaultString    = "\"\""
	DefaultInteger   = "0"
	DefaultDouble    = "0.0"
	DefaultFloat     = "0f"
	DefaultTimestamp = "ZonedDateTime.now()"
	DefaultList      = "emptyList()"
	DefaultMap       = "emptyMap()"
	DefaultLocalTime = "LocalTime.now()"

	version = "1.0.2"
)

var (
	id                  = "id"
	linebreakRegex      = regexp.MustCompile("[\r\n]+")
	kotlinDefaultValues = map[string]string{
		String:    DefaultString,
		Bool:      DefaultBoolFalse,
		Float:     DefaultFloat,
		Double:    DefaultDouble,
		Timestamp: DefaultTimestamp,
		Int:       DefaultInteger,
		Long:      DefaultInteger,
	}
	doubleSuffixes = map[string]struct{}{
		"lat":       {},
		"lon":       {},
		"latitude":  {},
		"longitude": {},
	}
	reservedKeywords = []string{"as", "as?", "break", "class", "continue", "do", "else", "false", "for", "fun", "if",
		"in", "!in", "interface", "is", "!is", "null", "object", "package", "return", "super", "this", "throw", "true",
		"try", "typealias", "typeof", "val", "var", "when", "while"}
)

type templateData struct {
	gen.GeneratorData
	Methods    []Method
	Models     []Model
	PackageAPI string
	Class      string
	Imports    []string
}

type Method struct {
	Name        string
	SafeName    string
	Description []string
	Errors      []Parameter
	Parameters  []Parameter
	Returns     Parameter
}

type Model struct {
	Name        string
	Description string
	IsInitial   bool
	Fields      []Parameter
}

type Parameter struct {
	Name             string
	Description      string
	Type             string
	BaseType         string
	ReturnType       string
	Optional         bool
	DefaultValue     string
	IsObject         bool
	DecodableDefault string
	Properties       []Parameter
}

type Settings struct {
	Class      string
	IsProtocol bool
	PackageAPI string
	Imports    []string
	TypeMapper TypeMapper
}

type TypeMapper func(typeName string, in smd.Property, kotlinType Parameter) Parameter

type Generator struct {
	schema   smd.Schema
	settings Settings
}

func NewClient(schema smd.Schema, settings Settings) *Generator {
	return &Generator{schema: schema, settings: settings}
}

// Generate return kotlin generated code by template
func (g *Generator) Generate() ([]byte, error) {
	data := g.prepareTemplateData()

	return g.executeTemplate(data)
}

// prepareTemplateData prepare template data
func (g *Generator) prepareTemplateData() templateData {
	if g.settings.PackageAPI == "" {
		g.settings.PackageAPI = BasePackageAPI
	}

	if g.settings.Class == "" {
		g.settings.Class = BaseClass
	}

	data := templateData{GeneratorData: gen.DefaultGeneratorData().AddLangAndLocalVersion(version, "kotlin"), PackageAPI: g.settings.PackageAPI, Imports: g.settings.Imports, Class: g.settings.Class}

	modelsMap := make(map[string]Model)

	for serviceName, service := range g.schema.Services {
		var (
			params []Parameter
		)
		desc := linebreakRegex.ReplaceAllString(service.Description, "\n")
		method := Method{
			Name:     serviceName,
			Errors:   g.prepareErrs(service.Errors),
			SafeName: strings.ReplaceAll(serviceName, ".", ""),
			Returns:  g.prepareParameter(service.Returns),
		}
		if desc != "" {
			method.Description = strings.Split(desc, "\n")
		}
		if len(service.Returns.Definitions) > 0 {
		}
		g.defToModelMap(modelsMap, service.Returns.Definitions)
		paramToModelMap(modelsMap, method.Returns)
		for _, param := range service.Parameters {
			p := g.prepareParameter(param)
			g.defToModelMap(modelsMap, param.Definitions)
			paramToModelMap(modelsMap, p)
			params = append(params, p)
		}
		method.Parameters = params
		normalizeMethodReturnType(&method)
		data.Methods = append(data.Methods, method)
	}

	for _, v := range modelsMap {
		data.Models = append(data.Models, v)
	}

	g.fillIsInitial(&data)
	g.sortTemplateData(&data)
	g.prepareModelFieldName(&data)

	g.resolveTypes(&data)
	return data
}

// normalizeMethodReturnType check suffix ids and type list convert to List<Long>
func normalizeMethodReturnType(method *Method) {
	if strings.HasSuffix(strings.ToLower(method.Name), "ids") && method.Returns.Type == "List<Int>" {
		method.Returns.Type = "List<Long>"
	}
}

func (g *Generator) fillIsInitial(data *templateData) {
	modelByName := make(map[string]*Model, len(data.Models))
	for i := range data.Models {
		modelByName[data.Models[i].Name] = &data.Models[i]
	}

	initialMap := make(map[string]struct{})

	// first step - search from all methods and find all model from method params
	for _, m := range data.Methods {
		for _, p := range m.Parameters {
			if p.IsObject {
				typ := p.Type
				if typ == "" {
					typ = p.ReturnType
				}
				checkModelName(typ, modelByName, initialMap)
			}
		}
	}

	// init loop for initialMap
	for loopWithChanged := true; loopWithChanged; {
		loopWithChanged = false
		for t := range initialMap {
			md := modelByName[t]
			if md == nil {
				continue
			}
			for _, f := range md.Fields {
				if f.IsObject {
					if _, ok := initialMap[f.Type]; !ok {
						// when find new modelName - use checkModelName and add next cycle
						checkModelName(f.Type, modelByName, initialMap)
						loopWithChanged = true
					}
				}
			}
		}
	}

	for name := range initialMap {
		if md := modelByName[name]; md != nil {
			md.IsInitial = true
		}
	}
}

func checkModelName(name string, modelByName map[string]*Model, initialMap map[string]struct{}) {
	if name == "" {
		return
	}
	if _, ok := modelByName[name]; !ok {
		return
	}
	initialMap[name] = struct{}{}
}

func (g *Generator) prepareModelFieldName(data *templateData) {
	for i := range data.Models {
		for j := range data.Models[i].Fields {
			data.Models[i].Fields[j].Name = formattedReservedKey(data.Models[i].Fields[j].Name)
		}
	}
}

func (g *Generator) sortTemplateData(data *templateData) {
	sort.Slice(data.Methods, func(i, j int) bool {
		return data.Methods[i].Name < data.Methods[j].Name
	})

	// sort errs by erName (it's  a number) because they was located in map
	for _, m := range data.Methods {
		sort.Slice(m.Errors, func(i, j int) bool {
			iNum, err := strconv.Atoi(m.Errors[i].Name)
			if err != nil {
				return m.Errors[i].Name < m.Errors[j].Name
			}
			jNum, err := strconv.Atoi(m.Errors[j].Name)
			if err != nil {
				return m.Errors[i].Name < m.Errors[j].Name
			}
			return iNum < jNum
		})
	}

	sort.Slice(data.Models, func(i, j int) bool {
		return data.Models[i].Name < data.Models[j].Name
	})

	for idx := range data.Models {
		sort.Slice(data.Models[idx].Fields, func(i, j int) bool {
			return data.Models[idx].Fields[i].Name < data.Models[idx].Fields[j].Name
		})
	}
}

func (g *Generator) executeTemplate(data templateData) ([]byte, error) {
	t := model
	if g.settings.IsProtocol {
		t = protocolTemplate
	}

	gen.TemplateFuncs["hasDescriptions"] = hasDescriptions

	tmpl, err := template.New("kotlin_client").Funcs(gen.TemplateFuncs).Parse(t)
	if err != nil {
		return nil, err
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return nil, err
	}

	return buf.Bytes(), nil
}

// resolveTypes last check and update params in methods
func (g *Generator) resolveTypes(data *templateData) {
	for i := range data.Methods {
		method := &data.Methods[i]
		for j := range method.Parameters {
			param := &method.Parameters[j]
			if newType, ok := inferKotlinType(param.Type, param.Name); ok {
				param.setType(newType)
			}
		}
	}
}

func inferKotlinType(valType, name string) (string, bool) {
	switch valType {
	case Int:
		if kotlinTypeID(name) {
			return Long, true
		}
	case Float:
		if kotlinTypeDouble(name) {
			return Double, true
		}
	case String:
		if kotlinTypeTimestamp(name) {
			return Timestamp, true
		}
	}
	return "", false
}

// setType update all types
func (p *Parameter) setType(t string) {
	p.Type = t
	p.BaseType = t
	p.ReturnType = t
}

// propertiesToParams convert smd.PropertyList to []Parameter
func (g *Generator) propertiesToParams(typeName string, list smd.PropertyList) []Parameter {
	parameters := make([]Parameter, 0, len(list))
	for _, prop := range list {
		p := Parameter{
			Name:        prop.Name,
			Optional:    prop.Optional,
			Description: prop.Description,
		}

		pType := kotlinType(prop.Type, p.Name)
		if prop.Type == smd.Object && prop.Ref != "" {
			pType = strings.TrimPrefix(prop.Ref, gen.DefinitionsPrefix)
			p.IsObject = true
		}

		if prop.Type == smd.Array {
			prop.Items["name"] = p.Name
			pType = arrayType(prop.Items, false)
		}

		p.Type = pType

		p.DefaultValue = kotlinDefault(pType)

		if g.settings.TypeMapper != nil {
			p = g.settings.TypeMapper(typeName, prop, p)
		}
		parameters = append(parameters, p)
	}
	return parameters
}

// prepareErrs convert errs map to []Parameter
func (g *Generator) prepareErrs(errs map[int]string) []Parameter {
	pp := make([]Parameter, 0, len(errs))
	for code, err := range errs {
		pp = append(pp, Parameter{
			Name:        strconv.Itoa(code),
			Description: err,
		})
	}

	return pp
}

// prepareParameter create Parameter from smd.JSONSchema
func (g *Generator) prepareParameter(in smd.JSONSchema) Parameter {
	out := Parameter{
		Name:        in.Name,
		Description: in.Description,
		BaseType:    kotlinType(in.Type),
		Optional:    in.Optional,
		Properties:  g.propertiesToParams(in.TypeName, in.Properties),
	}

	pType := out.BaseType
	out.ReturnType = pType
	if in.Type == smd.Object {
		typeName := in.TypeName
		if typeName == "" && in.Description != "" && smd.IsSMDTypeName(in.Description, in.Type) {
			typeName = in.Description
		}

		if typeName != "" {
			pType = typeName
			out.ReturnType = pType

		}
		out.IsObject = true
	}
	out.Type = pType

	if in.Type == smd.Array {
		in.Items["name"] = in.Name
		out.Type = arrayType(in.Items, false)
		out.ReturnType = arrayType(in.Items, true)
	}

	return out
}

// defToModelMap convert smd.Definition to Model and add to models map
func (g *Generator) defToModelMap(modelMap map[string]Model, definitions map[string]smd.Definition) {
	for name, def := range definitions {
		modelMap[name] = Model{Name: name, Fields: g.propertiesToParams(name, def.Properties)}
	}
}

// paramToModelMap add Parameter to model map if parameter is object
func paramToModelMap(modelsMap map[string]Model, p Parameter) {
	if p.IsObject && p.BaseType != p.Type {
		modelsMap[p.Type] = Model{
			Name:   p.Type,
			Fields: p.Properties,
		}
	}
}

// kotlinType convert smd types to kotlin types
func kotlinType(smdType string, propNames ...string) string {
	var name string
	if len(propNames) > 0 {
		name = propNames[0]
	}

	switch smdType {
	case smd.String:
		if kotlinTypeTimestamp(name) {
			return Timestamp
		}
		return String
	case smd.Boolean:
		return Bool
	case smd.Float:
		if kotlinTypeDouble(name) {
			return Double
		}
		return Float
	case smd.Integer:
		if kotlinTypeID(name) {
			return Long
		}
		return Int
	}
	return smdType
}

// kotlinDefault convert smd types to kotlin default types
func kotlinDefault(smdType string) string {
	if val, ok := kotlinDefaultValues[smdType]; ok {
		return val
	}

	if strings.HasPrefix(smdType, List) {
		return DefaultList
	}

	if strings.HasPrefix(smdType, Map) {
		return DefaultMap
	}

	return smdType
}

// kotlinTypeDouble check if property need set type Double
func kotlinTypeDouble(name string) bool {
	name = strings.ToLower(name)
	if _, ok := doubleSuffixes[name]; ok {
		return true
	}

	for suffix := range doubleSuffixes {
		if strings.HasSuffix(name, suffix) {
			return true
		}
	}

	return false
}

// kotlinTypeID check if property is ID or IDs set type Long
func kotlinTypeID(name string) bool {
	return name == id ||
		strings.HasSuffix(name, strings.ToUpper(id)) ||
		strings.HasSuffix(name, "Id") ||
		strings.HasSuffix(name, "Id"+"s") ||
		strings.HasSuffix(name, strings.ToUpper(id)+"s")
}

// kotlinTypeTimestamp check if is time property set Timestamp
func kotlinTypeTimestamp(name string) bool {
	return strings.HasSuffix(name, "At")
}

func arrayType(items map[string]string, isReturnType bool) string {
	var subType string
	if scalar, ok := items["type"]; ok {
		subType = kotlinType(scalar, items["name"])
	}
	if ref, ok := items["$ref"]; ok {
		subType = strings.TrimPrefix(ref, gen.DefinitionsPrefix)
	}

	if isReturnType {
		return subType
	}

	return fmt.Sprintf("List<%s>", subType)
}

// hasDescriptions check description
func hasDescriptions(m Method) bool {
	return m.Returns.Description != "" || len(m.Description) > 0 || len(m.Parameters) > 0
}

// formattedReservedKey formatted if key exist in reservedKeywords
func formattedReservedKey(str string) string {
	for _, keyword := range reservedKeywords {
		if str == keyword {
			return fmt.Sprintf("`%s`", str)
		}
	}
	return str
}
