package parser

import (
	"go/ast"
	"log"
	"reflect"
	"strings"
)

func (pi *PackageInfo) parseStructs() {
	for _, s := range pi.Structs {
		if err := s.parse(pi, []string{}); err != nil {
			log.Printf("Error parsing struct %s: %v", s.Name, err)
		}
	}
}

func (s *Struct) findTypeSpec(pi *PackageInfo) bool {
	if s.StructType != nil {
		return true
	}

	for _, f := range pi.Scopes[s.Namespace] {
		if obj, ok := f.Objects[s.Type]; ok && obj.Decl != nil {
			if ts, ok := obj.Decl.(*ast.TypeSpec); ok {
				if st, ok := getFinalType(ts.Type).(*ast.StructType); ok {
					s.StructType = st
					return true
				}
			}
		}
	}

	return false
}

// parse parses package
//
//nolint:gocyclo,gocognit
func (s *Struct) parse(pi *PackageInfo, parsed []string) error {
	if !s.findTypeSpec(pi) || s.Properties != nil {
		// can't find struct implementation
		// or struct already parsed
		return nil
	}

	s.Properties = []Property{}
	parsed = append(parsed, s.Name)
	for _, field := range s.StructType.Fields.List {
		tag := parseJSONTag(field.Tag)

		// do not parse tags that ignored in json
		if tag == "-" {
			continue
		}

		// parse embedded struct
		if field.Names == nil {
			if embeddedS := parseStruct(field.Type); embeddedS != nil {
				// set right namespace for struct from another package
				if embeddedS.Namespace == "." && s.Namespace != "." {
					embeddedS.Namespace = s.Namespace
					embeddedS.Name = s.Namespace + "." + embeddedS.Type
				}

				if currentS, ok := pi.Structs[embeddedS.Name]; !ok || (currentS.StructType == nil && embeddedS.StructType != nil) {
					pi.Structs[embeddedS.Name] = embeddedS
				}

				if err := embeddedS.parse(pi, parsed); err != nil {
					return err
				}

				if len(embeddedS.Properties) > 0 {
					s.Properties = append(s.Properties, embeddedS.Properties...)
				}
			}

			continue
		}

		smdType, itemType := parseSMDType(field.Type)
		hasStar := hasStar(parseType(field.Type))

		var ref string
		// parse field with struct type
		if internalS := parseStruct(field.Type); internalS != nil {
			// set right namespace for struct from another package
			if internalS.Namespace == "." && s.Namespace != "." {
				internalS.Namespace = s.Namespace
				internalS.Name = s.Namespace + "." + internalS.Type
			}

			ref = internalS.Name
			if currentS, ok := pi.Structs[internalS.Name]; !ok || (currentS.StructType == nil && internalS.StructType != nil) {
				pi.Structs[internalS.Name] = internalS
			}

			// avoid self-linked infinite recursion
			if !hasRef(internalS.Name, parsed) {
				if err := internalS.parse(pi, parsed); err != nil {
					return err
				}
			}
		}

		// parse inline struct
		if inlineStructType, ok := field.Type.(*ast.StructType); ok {
			// call struct by first property name
			inlineS := &Struct{
				Name:       s.Name + "_" + field.Names[0].Name,
				Namespace:  s.Namespace,
				Type:       s.Type + "_" + field.Names[0].Name,
				StructType: inlineStructType,
			}

			pi.Structs[inlineS.Name] = inlineS
			ref = inlineS.Name
			if err := inlineS.parse(pi, parsed); err != nil {
				return err
			}
		}

		// description
		description := parseCommentGroup(field.Doc)
		comment := parseCommentGroup(field.Comment)
		if description != "" && comment != "" {
			description += "\n"
		}
		description += comment

		// parse names
		for i, name := range field.Names {
			if !ast.IsExported(name.Name) {
				continue
			}

			p := Property{
				Name:        name.Name,
				HasStar:     hasStar,
				Description: description,
				SMDType: SMDType{
					Type:      smdType,
					ItemsType: itemType,
					Ref:       ref,
				},
			}

			if i == 0 {
				// tag only for first name
				if tag == "-" {
					continue
				} else if tag != "" {
					p.Name = tag
				}
			}

			s.Properties = append(s.Properties, p)
		}
	}

	return nil
}

func parseJSONTag(bl *ast.BasicLit) string {
	if bl == nil {
		return ""
	}

	tags := bl.Value[1 : len(bl.Value)-1] // remove quotes ``
	tag := strings.Split(reflect.StructTag(tags).Get("json"), ",")[0]

	return tag
}

// Definitions returns list of structs used inside smdType
func Definitions(smdType SMDType, structs map[string]*Struct) []*Struct {
	var defs []string

	// array of objects
	if smdType.Type == SmdArray && smdType.ItemsType == SmdObject {
		defs = append(defs, smdType.Ref)
	}

	if s, ok := structs[smdType.Ref]; ok {
		for _, p := range s.Properties {
			defs = definitions(p.SMDType, structs, defs)
		}
	}

	var result []*Struct
	unique := map[string]struct{}{} // structs in result must be unique
	for _, name := range defs {
		if s, ok := structs[name]; ok {
			if _, ok := unique[name]; !ok {
				result = append(result, s)
				unique[name] = struct{}{}
			}
		}
	}

	return result
}

// definitions returns list of struct names used inside smdType
func definitions(smdType SMDType, structs map[string]*Struct, defs []string) []string {
	if smdType.Ref == "" || hasRef(smdType.Ref, defs) {
		return defs
	}

	// array of objects
	if (smdType.Type == SmdArray && smdType.ItemsType == SmdObject) || smdType.Type == SmdObject {
		defs = append(defs, smdType.Ref)
	}

	if s, ok := structs[smdType.Ref]; ok {
		for _, p := range s.Properties {
			defs = definitions(p.SMDType, structs, defs)
		}
	}

	return defs
}

func hasRef(needle string, haystack []string) bool {
	for _, n := range haystack {
		if n == needle {
			return true
		}
	}

	return false
}
