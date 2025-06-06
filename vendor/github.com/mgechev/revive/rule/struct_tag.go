package rule

import (
	"fmt"
	"go/ast"
	"strconv"
	"strings"

	"github.com/fatih/structtag"
	"github.com/mgechev/revive/lint"
)

// StructTagRule lints struct tags.
type StructTagRule struct {
	userDefined map[string][]string // map: key -> []option
}

// Configure validates the rule configuration, and configures the rule accordingly.
//
// Configuration implements the [lint.ConfigurableRule] interface.
func (r *StructTagRule) Configure(arguments lint.Arguments) error {
	if len(arguments) == 0 {
		return nil
	}

	err := checkNumberOfArguments(1, arguments, r.Name())
	if err != nil {
		return err
	}
	r.userDefined = make(map[string][]string, len(arguments))
	for _, arg := range arguments {
		item, ok := arg.(string)
		if !ok {
			return fmt.Errorf("invalid argument to the %s rule. Expecting a string, got %v (of type %T)", r.Name(), arg, arg)
		}
		parts := strings.Split(item, ",")
		if len(parts) < 2 {
			return fmt.Errorf("invalid argument to the %s rule. Expecting a string of the form key[,option]+, got %s", r.Name(), item)
		}
		key := strings.TrimSpace(parts[0])
		for i := 1; i < len(parts); i++ {
			option := strings.TrimSpace(parts[i])
			r.userDefined[key] = append(r.userDefined[key], option)
		}
	}
	return nil
}

// Apply applies the rule to given file.
func (r *StructTagRule) Apply(file *lint.File, _ lint.Arguments) []lint.Failure {
	var failures []lint.Failure
	onFailure := func(failure lint.Failure) {
		failures = append(failures, failure)
	}

	w := lintStructTagRule{
		onFailure:      onFailure,
		userDefined:    r.userDefined,
		isAtLeastGo124: file.Pkg.IsAtLeastGo124(),
	}

	ast.Walk(w, file.AST)

	return failures
}

// Name returns the rule name.
func (*StructTagRule) Name() string {
	return "struct-tag"
}

type lintStructTagRule struct {
	onFailure      func(lint.Failure)
	userDefined    map[string][]string // map: key -> []option
	usedTagNbr     map[int]bool        // list of used tag numbers
	usedTagName    map[string]bool     // list of used tag keys
	isAtLeastGo124 bool
}

func (w lintStructTagRule) Visit(node ast.Node) ast.Visitor {
	switch n := node.(type) {
	case *ast.StructType:
		isEmptyStruct := n.Fields == nil || n.Fields.NumFields() < 1
		if isEmptyStruct {
			return nil // skip empty structs
		}

		w.usedTagNbr = map[int]bool{}
		w.usedTagName = map[string]bool{}
		for _, f := range n.Fields.List {
			if f.Tag != nil {
				w.checkTaggedField(f)
			}
		}
	}

	return w
}

const (
	keyASN1     = "asn1"
	keyBSON     = "bson"
	keyDefault  = "default"
	keyJSON     = "json"
	keyProtobuf = "protobuf"
	keyRequired = "required"
	keyXML      = "xml"
	keyYAML     = "yaml"
)

func (w lintStructTagRule) checkTagNameIfNeed(tag *structtag.Tag) (string, bool) {
	isUnnamedTag := tag.Name == "" || tag.Name == "-"
	if isUnnamedTag {
		return "", true
	}

	switch tag.Key {
	case keyBSON, keyJSON, keyXML, keyYAML, keyProtobuf:
	default:
		return "", true
	}

	tagName := w.getTagName(tag)
	if tagName == "" {
		return "", true // No tag name found
	}

	// We concat the key and name as the mapping key here
	// to allow the same tag name in different tag type.
	key := tag.Key + ":" + tagName
	if _, ok := w.usedTagName[key]; ok {
		return fmt.Sprintf("duplicate tag name: '%s'", tagName), false
	}

	w.usedTagName[key] = true

	return "", true
}

func (lintStructTagRule) getTagName(tag *structtag.Tag) string {
	switch tag.Key {
	case keyProtobuf:
		for _, option := range tag.Options {
			if tagName, found := strings.CutPrefix(option, "name="); found {
				return tagName
			}
		}
		return "" // protobuf tag lacks 'name' option
	default:
		return tag.Name
	}
}

// checkTaggedField checks the tag of the given field.
// precondition: the field has a tag
func (w lintStructTagRule) checkTaggedField(f *ast.Field) {
	if len(f.Names) > 0 && !f.Names[0].IsExported() {
		w.addFailure(f, "tag on not-exported field "+f.Names[0].Name)
	}

	tags, err := structtag.Parse(strings.Trim(f.Tag.Value, "`"))
	if err != nil || tags == nil {
		w.addFailure(f.Tag, "malformed tag")
		return
	}

	for _, tag := range tags.Tags() {
		if msg, ok := w.checkTagNameIfNeed(tag); !ok {
			w.addFailure(f.Tag, msg)
		}

		switch key := tag.Key; key {
		case keyASN1:
			msg, ok := w.checkASN1Tag(f.Type, tag)
			if !ok {
				w.addFailure(f.Tag, msg)
			}
		case keyBSON:
			msg, ok := w.checkBSONTag(tag.Options)
			if !ok {
				w.addFailure(f.Tag, msg)
			}
		case keyDefault:
			if !w.typeValueMatch(f.Type, tag.Name) {
				w.addFailure(f.Tag, "field's type and default value's type mismatch")
			}
		case keyJSON:
			msg, ok := w.checkJSONTag(tag.Name, tag.Options)
			if !ok {
				w.addFailure(f.Tag, msg)
			}
		case keyProtobuf:
			msg, ok := w.checkProtobufTag(tag)
			if !ok {
				w.addFailure(f.Tag, msg)
			}
		case keyRequired:
			if tag.Name != "true" && tag.Name != "false" {
				w.addFailure(f.Tag, "required should be 'true' or 'false'")
			}
		case keyXML:
			msg, ok := w.checkXMLTag(tag.Options)
			if !ok {
				w.addFailure(f.Tag, msg)
			}
		case keyYAML:
			msg, ok := w.checkYAMLTag(tag.Options)
			if !ok {
				w.addFailure(f.Tag, msg)
			}
		default:
			// unknown key
		}
	}
}

func (w lintStructTagRule) checkASN1Tag(t ast.Expr, tag *structtag.Tag) (string, bool) {
	checkList := append(tag.Options, tag.Name)
	for _, opt := range checkList {
		switch opt {
		case "application", "explicit", "generalized", "ia5", "omitempty", "optional", "set", "utf8":

		default:
			if strings.HasPrefix(opt, "tag:") {
				parts := strings.Split(opt, ":")
				tagNumber := parts[1]
				number, err := strconv.Atoi(tagNumber)
				if err != nil {
					return fmt.Sprintf("ASN1 tag must be a number, got '%s'", tagNumber), false
				}
				if w.usedTagNbr[number] {
					return fmt.Sprintf("duplicated tag number %v", number), false
				}
				w.usedTagNbr[number] = true

				continue
			}

			if strings.HasPrefix(opt, "default:") {
				parts := strings.Split(opt, ":")
				if len(parts) < 2 {
					return "malformed default for ASN1 tag", false
				}
				if !w.typeValueMatch(t, parts[1]) {
					return "field's type and default value's type mismatch", false
				}

				continue
			}

			if w.isUserDefined(keyASN1, opt) {
				continue
			}

			return fmt.Sprintf("unknown option '%s' in ASN1 tag", opt), false
		}
	}

	return "", true
}

func (w lintStructTagRule) checkBSONTag(options []string) (string, bool) {
	for _, opt := range options {
		switch opt {
		case "inline", "minsize", "omitempty":
		default:
			if w.isUserDefined(keyBSON, opt) {
				continue
			}
			return fmt.Sprintf("unknown option '%s' in BSON tag", opt), false
		}
	}

	return "", true
}

func (w lintStructTagRule) checkJSONTag(name string, options []string) (string, bool) {
	for _, opt := range options {
		switch opt {
		case "omitempty", "string":
		case "":
			// special case for JSON key "-"
			if name != "-" {
				return "option can not be empty in JSON tag", false
			}
		case "omitzero":
			if w.isAtLeastGo124 {
				continue
			}
			fallthrough
		default:
			if w.isUserDefined(keyJSON, opt) {
				continue
			}
			return fmt.Sprintf("unknown option '%s' in JSON tag", opt), false
		}
	}

	return "", true
}

func (w lintStructTagRule) checkXMLTag(options []string) (string, bool) {
	for _, opt := range options {
		switch opt {
		case "any", "attr", "cdata", "chardata", "comment", "innerxml", "omitempty", "typeattr":
		default:
			if w.isUserDefined(keyXML, opt) {
				continue
			}
			return fmt.Sprintf("unknown option '%s' in XML tag", opt), false
		}
	}

	return "", true
}

func (w lintStructTagRule) checkYAMLTag(options []string) (string, bool) {
	for _, opt := range options {
		switch opt {
		case "flow", "inline", "omitempty":
		default:
			if w.isUserDefined(keyYAML, opt) {
				continue
			}
			return fmt.Sprintf("unknown option '%s' in YAML tag", opt), false
		}
	}

	return "", true
}

func (lintStructTagRule) typeValueMatch(t ast.Expr, val string) bool {
	tID, ok := t.(*ast.Ident)
	if !ok {
		return true
	}

	typeMatches := true
	switch tID.Name {
	case "bool":
		typeMatches = val == "true" || val == "false"
	case "float64":
		_, err := strconv.ParseFloat(val, 64)
		typeMatches = err == nil
	case "int":
		_, err := strconv.ParseInt(val, 10, 64)
		typeMatches = err == nil
	case "string":
	case "nil":
	default:
		// unchecked type
	}

	return typeMatches
}

func (w lintStructTagRule) checkProtobufTag(tag *structtag.Tag) (string, bool) {
	// check name
	switch tag.Name {
	case "bytes", "fixed32", "fixed64", "group", "varint", "zigzag32", "zigzag64":
		// do nothing
	default:
		return fmt.Sprintf("invalid protobuf tag name '%s'", tag.Name), false
	}

	// check options
	seenOptions := map[string]bool{}
	for _, opt := range tag.Options {
		if number, err := strconv.Atoi(opt); err == nil {
			_, alreadySeen := w.usedTagNbr[number]
			if alreadySeen {
				return fmt.Sprintf("duplicated tag number %v", number), false
			}
			w.usedTagNbr[number] = true
			continue // option is an integer
		}

		switch {
		case opt == "opt" || opt == "proto3" || opt == "rep" || opt == "req":
			// do nothing
		case strings.Contains(opt, "="):
			o := strings.Split(opt, "=")[0]
			_, alreadySeen := seenOptions[o]
			if alreadySeen {
				return fmt.Sprintf("protobuf tag has duplicated option '%s'", o), false
			}
			seenOptions[o] = true
			continue
		}
	}
	_, hasName := seenOptions["name"]
	if !hasName {
		return "protobuf tag lacks mandatory option 'name'", false
	}

	for k := range seenOptions {
		switch k {
		case "name", "json":
			// do nothing
		default:
			if w.isUserDefined(keyProtobuf, k) {
				continue
			}
			return fmt.Sprintf("unknown option '%s' in protobuf tag", k), false
		}
	}

	return "", true
}

func (w lintStructTagRule) addFailure(n ast.Node, msg string) {
	w.onFailure(lint.Failure{
		Node:       n,
		Failure:    msg,
		Confidence: 1,
	})
}

func (w lintStructTagRule) isUserDefined(key, opt string) bool {
	if w.userDefined == nil {
		return false
	}

	options := w.userDefined[key]
	for _, o := range options {
		if opt == o {
			return true
		}
	}
	return false
}
