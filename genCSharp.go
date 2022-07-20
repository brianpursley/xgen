// Copyright 2020 - 2022 The xgen Authors. All rights reserved. Use of this
// source code is governed by a BSD-style license that can be found in the
// LICENSE file.
//
// Package xgen written in pure Go providing a set of functions that allow you
// to parse XSD (XML schema files). This library needs Go version 1.10 or
// later.

package xgen

import (
	"fmt"
	"os"
	"reflect"
	"strings"
)

var csharpBuildInType = map[string]bool{
	"bool":         true,
	"byte":         true,
	"byte[]":       true,
	"DateOnly":     true,
	"DateTime":     true,
	"decimal":      true,
	"double":       true,
	"float":        true,
	"int":          true,
	"List<string>": true,
	"long":         true,
	"object":       true,
	"short":        true,
	"string":       true,
	"TimeOnly":     true,
	"uint":         true,
	"ulong":        true,
	"ushort":       true,
}

var csharpBuildInDefaultValues = map[string]string{
	"byte[]":       "Array.Empty<byte>()",
	"List<string>": "new()",
	"object":       "new()",
	"string":       "\"\"",
}

// GenCSharp generate CSharp programming language source code for XML schema
// definition files.
func (gen *CodeGenerator) GenCSharp() error {
	fieldNameCount = make(map[string]int)
	for _, ele := range gen.ProtoTree {
		if ele == nil {
			continue
		}
		funcName := fmt.Sprintf("CSharp%s", reflect.TypeOf(ele).String()[6:])
		callFuncByName(gen, funcName, []reflect.Value{reflect.ValueOf(ele)})
	}
	f, err := os.Create(gen.File + ".cs")
	if err != nil {
		return err
	}
	defer f.Close()
	ns := gen.Package
	if ns == "" {
		ns = "schema"
	}
	var using = `using System.CodeDom.Compiler;
using System.Xml.Serialization;
`

	f.Write([]byte(fmt.Sprintf("%s\n\n%s\nnamespace %s\n{%s}\n", copyright, using, ns, gen.Field)))
	return err
}

func genCSharpFieldName(name string, unique bool) (fieldName string) {
	for _, str := range strings.Split(name, ":") {
		fieldName += MakeFirstUpperCase(str)
	}
	var tmp string
	for _, str := range strings.Split(fieldName, ".") {
		tmp += MakeFirstUpperCase(str)
	}
	fieldName = tmp
	fieldName = strings.Replace(fieldName, "-", "", -1)
	if unique {
		fieldNameCount[fieldName]++
		if count := fieldNameCount[fieldName]; count != 1 {
			fieldName = fmt.Sprintf("%s%d", fieldName, count)
		}
	}
	return
}

func genCSharpFieldType(name string) string {
	if _, ok := csharpBuildInType[name]; ok {
		return name
	}
	var fieldType string
	for _, str := range strings.Split(name, ".") {
		fieldType += MakeFirstUpperCase(str)
	}
	fieldType = MakeFirstUpperCase(strings.Replace(fieldType, "-", "", -1))
	if fieldType != "" {
		return fieldType
	}
	return "object"
}

// CSharpSimpleType generates code for simple type XML schema in CSharp language
// syntax.
func (gen *CodeGenerator) CSharpSimpleType(v *SimpleType) {
	if v.List {
		if _, ok := gen.StructAST[v.Name]; !ok {
			fieldType := genCSharpFieldType(getBasefromSimpleType(trimNSPrefix(v.Base), gen.ProtoTree))
			content := fmt.Sprintf(" : List<%s> {};\n", fieldType)
			gen.StructAST[v.Name] = content
			fieldName := genCSharpFieldName(v.Name, true)
			gen.Field += fmt.Sprintf("%s%s\tpublic partial class %s%s", genFieldComment(fieldName, v.Doc, "\t//"), genCSharpClassAttributes(v.Name), fieldName, content)
			return
		}
	}
	if v.Union && len(v.MemberTypes) > 0 {
		if _, ok := gen.StructAST[v.Name]; !ok {
			content := "\n\t{\n"
			for _, member := range toSortedPairs(v.MemberTypes) {
				memberName := member.key
				memberType := member.value

				if memberType == "" { // fix order issue
					memberType = getBasefromSimpleType(memberName, gen.ProtoTree)
				}
				fieldType := genCSharpFieldType(memberType)
				content += fmt.Sprintf("\t\t%s public %s? %s { get; set; }\n", genCSharpFieldAttributes(memberName, true), fieldType, genCSharpFieldName(memberName, false))
			}
			content += "\t}\n"
			gen.StructAST[v.Name] = content
			fieldName := genCSharpFieldName(v.Name, true)
			gen.Field += fmt.Sprintf("%s%s\tpublic partial class %s%s", genFieldComment(fieldName, v.Doc, "\t//"), genCSharpClassAttributes(v.Name), fieldName, content)
		}
		return
	}
	if _, ok := gen.StructAST[v.Name]; !ok {
		fieldType := genCSharpFieldType(getBasefromSimpleType(trimNSPrefix(v.Base), gen.ProtoTree))
		if !isBuiltInCSharpType(fieldType) {
			content := fmt.Sprintf(" : %s {};\n", fieldType)
			gen.StructAST[v.Name] = content
			fieldName := genCSharpFieldName(v.Name, true)
			gen.Field += fmt.Sprintf("%s%s\tpublic partial class %s%s", genFieldComment(fieldName, v.Doc, "\t//"), genCSharpClassAttributes(v.Name), fieldName, content)
			return
		}
	}
}

// CSharpComplexType generates code for complex type XML schema in CSharp language
// syntax.
func (gen *CodeGenerator) CSharpComplexType(v *ComplexType) {
	if _, ok := gen.StructAST[v.Name]; !ok {
		className := genCSharpFieldName(v.Name, true)

		content := "\n\t{\n"
		for _, attrGroup := range v.AttributeGroup {
			fieldType := getBasefromSimpleType(trimNSPrefix(attrGroup.Ref), gen.ProtoTree)
			fieldName := genCSharpFieldName(attrGroup.Name, false)
			if fieldName == className {
				fieldName += "Value"
			}
			content += fmt.Sprintf("\t%s public %s? %s { get; set; }\n", genCSharpFieldAttributes(attrGroup.Name, true), genCSharpFieldType(fieldType), fieldName)
		}

		for _, attribute := range v.Attributes {
			fieldType := genCSharpFieldType(getBasefromSimpleType(trimNSPrefix(attribute.Type), gen.ProtoTree))
			if attribute.Optional {
				fieldType += "?"
			}
			fieldName := genCSharpFieldName(attribute.Name, false)
			content += fmt.Sprintf("\t%s public %s %sAttr { get; set; }%s\n", genCSharpFieldAttributes(attribute.Name, false), fieldType, fieldName, getCSharpDefaultValue(fieldType))
		}
		for _, group := range v.Groups {
			var fieldType = genCSharpFieldType(getBasefromSimpleType(trimNSPrefix(group.Ref), gen.ProtoTree))
			if group.Plural {
				fieldType = fmt.Sprintf("List<%s>", fieldType)
			}
			fieldName := genCSharpFieldName(group.Name, false)
			if fieldName == className {
				fieldName += "Value"
			}
			content += fmt.Sprintf("\tpublic %s? %s { get; set; }\n", fieldType, fieldName)
		}

		for _, element := range v.Elements {
			fieldType := genCSharpFieldType(getBasefromSimpleType(trimNSPrefix(element.Type), gen.ProtoTree))
			if element.Plural {
				fieldType = fmt.Sprintf("List<%s>", fieldType)
			}
			if element.Optional {
				fieldType += "?"
			}
			fieldName := genCSharpFieldName(element.Name, false)
			if fieldName == className {
				fieldName += "Value"
			}
			content += fmt.Sprintf("\t%s public %s %s { get; set; }%s\n", genCSharpFieldAttributes(element.Name, true), fieldType, fieldName, getCSharpDefaultValue(fieldType))
		}

		if len(v.Base) > 0 && isBuiltInCSharpType(v.Base) {
			fieldType := genCSharpFieldType(getBasefromSimpleType(trimNSPrefix(v.Base), gen.ProtoTree))
			content += fmt.Sprintf("\t\t[XmlText(typeof(%s))] public %s? Value { get; set; }\n", fieldType, fieldType)
		}

		content += "\t}\n"
		gen.StructAST[v.Name] = content

		inheritance := ""
		if len(v.Base) > 0 && !isBuiltInCSharpType(v.Base) {
			fieldType := genCSharpFieldType(getBasefromSimpleType(trimNSPrefix(v.Base), gen.ProtoTree))
			inheritance = fmt.Sprintf(" : %s ", fieldType)
		}

		gen.Field += fmt.Sprintf("%s%s\tpublic partial class %s%s%s", genFieldComment(className, v.Doc, "\t//"), genCSharpClassAttributes(v.Name), className, inheritance, gen.StructAST[v.Name])
	}
}

func isBuiltInCSharpType(typeName string) bool {
	_, builtIn := csharpBuildInType[typeName]
	return builtIn
}

// CSharpGroup generates code for group XML schema in CSharp language syntax.
func (gen *CodeGenerator) CSharpGroup(v *Group) {
	if _, ok := gen.StructAST[v.Name]; !ok {
		content := "\n\t{\n"
		for _, element := range v.Elements {
			fieldType := genCSharpFieldType(getBasefromSimpleType(trimNSPrefix(element.Type), gen.ProtoTree))
			if element.Plural {
				fieldType = fmt.Sprintf("List<%s>", fieldType)
			}
			if element.Optional {
				fieldType += "?"
			}
			content += fmt.Sprintf("\t\t%s public %s %s { get; set; }%s\n", genCSharpFieldAttributes(element.Name, true), fieldType, genCSharpFieldName(element.Name, false), getCSharpDefaultValue(fieldType))
		}

		for _, group := range v.Groups {
			fieldType := genCSharpFieldType(getBasefromSimpleType(trimNSPrefix(group.Ref), gen.ProtoTree))
			if group.Plural {
				fieldType = fmt.Sprintf("List<%s>", fieldType)
			}
			content += fmt.Sprintf("\t\t%s public %s? %s { get; set; }\n", genCSharpFieldAttributes(group.Name, true), fieldType, genCSharpFieldName(group.Name, false))
		}

		content += "\t}\n"
		gen.StructAST[v.Name] = content
		fieldName := genCSharpFieldName(v.Name, true)
		gen.Field += fmt.Sprintf("%s%s\tpublic partial class %s%s", genFieldComment(fieldName, v.Doc, "\t//"), genCSharpClassAttributes(v.Name), fieldName, gen.StructAST[v.Name])
	}
}

// CSharpAttributeGroup generates code for attribute group XML schema in CSharp language
// syntax.
func (gen *CodeGenerator) CSharpAttributeGroup(v *AttributeGroup) {
	if _, ok := gen.StructAST[v.Name]; !ok {
		className := genCSharpFieldName(v.Name, true)
		content := "\n\t{\n"
		for _, attribute := range v.Attributes {
			fieldType := genCSharpFieldType(getBasefromSimpleType(trimNSPrefix(attribute.Type), gen.ProtoTree))
			if attribute.Optional {
				fieldType += "?"
			}
			fieldName := genCSharpFieldName(attribute.Name, false)
			content += fmt.Sprintf("\t\t%s public %s %sAttr { get; set; }%s\n", genCSharpFieldAttributes(attribute.Name, false), fieldType, fieldName, getCSharpDefaultValue(fieldType))
		}
		content += "\t}\n"
		gen.StructAST[v.Name] = content
		gen.Field += fmt.Sprintf("%s%s\tpublic partial class %s%s", genFieldComment(className, v.Doc, "\t//"), genCSharpClassAttributes(v.Name), className, gen.StructAST[v.Name])
	}
}

// CSharpElement generates code for element XML schema in CSharp language syntax.
func (gen *CodeGenerator) CSharpElement(v *Element) {
	if _, ok := gen.StructAST[v.Name]; !ok {
		var fieldType = genCSharpFieldType(getBasefromSimpleType(trimNSPrefix(v.Type), gen.ProtoTree))
		if !isBuiltInCSharpType(fieldType) {
			if v.Plural {
				fieldType = fmt.Sprintf("List<%s>", fieldType)
			}
			content := fmt.Sprintf(" : %s {};\n", fieldType)
			gen.StructAST[v.Name] = content
			fieldName := genCSharpFieldName(v.Name, true)
			gen.Field += fmt.Sprintf("%s%s\tpublic partial class %s%s", genFieldComment(fieldName, v.Doc, "\t//"), genCSharpClassAttributes(v.Name), fieldName, content)
		}
	}
}

// CSharpAttribute generates code for attribute XML schema in CSharp language syntax.
func (gen *CodeGenerator) CSharpAttribute(v *Attribute) {
	if _, ok := gen.StructAST[v.Name]; !ok {
		var fieldType = genCSharpFieldType(getBasefromSimpleType(trimNSPrefix(v.Type), gen.ProtoTree))
		if !isBuiltInCSharpType(fieldType) {
			if v.Plural {
				fieldType = fmt.Sprintf("List<%s>", fieldType)
			}
			content := fmt.Sprintf(" : %s {};\n", fieldType)
			gen.StructAST[v.Name] = content
			fieldName := genCSharpFieldName(v.Name, true)
			gen.Field += fmt.Sprintf("%s%s\tpublic partial class %s%s", genFieldComment(fieldName, v.Doc, "\t//"), genCSharpClassAttributes(v.Name), fieldName, content)
		}
	}
}

func genCSharpClassAttributes(name string) string {
	return fmt.Sprintf("\t[GeneratedCode(\"xgen\", null)]\n\t[XmlType(\"%s\")]\n", name)
}

func genCSharpFieldAttributes(name string, isElement bool) string {
	if isElement {
		return fmt.Sprintf("\t[XmlElement(\"%s\")]", name)
	}
	return fmt.Sprintf("\t[XmlAttribute(\"%s\")]", name)
}

func getCSharpDefaultValue(typeName string) string {
	if strings.HasSuffix(typeName, "?") {
		return ""
	}
	if isBuiltInCSharpType(typeName) {
		if defaultValue, ok := csharpBuildInDefaultValues[typeName]; ok {
			return fmt.Sprintf(" = %s;", defaultValue)
		}
		return ""
	}
	return " = new();"
}
