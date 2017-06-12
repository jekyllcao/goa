package codegen

import (
	"bytes"
	"errors"
	"fmt"
	"strings"
	"text/template"

	"goa.design/goa.v2/design"
)

var (
	enumValT     *template.Template
	formatValT   *template.Template
	patternValT  *template.Template
	minMaxValT   *template.Template
	lengthValT   *template.Template
	requiredValT *template.Template
	arrayValT    *template.Template
	mapValT      *template.Template
	userValT     *template.Template
)

func init() {
	fm := template.FuncMap{
		"slice":    toSlice,
		"oneof":    oneof,
		"constant": constant,
		"goifyAtt": GoifyAtt,
		"add":      func(a, b int) int { return a + b },
	}
	enumValT = template.Must(template.New("enum").Funcs(fm).Parse(enumValTmpl))
	formatValT = template.Must(template.New("format").Funcs(fm).Parse(formatValTmpl))
	patternValT = template.Must(template.New("pattern").Funcs(fm).Parse(patternValTmpl))
	minMaxValT = template.Must(template.New("minMax").Funcs(fm).Parse(minMaxValTmpl))
	lengthValT = template.Must(template.New("length").Funcs(fm).Parse(lengthValTmpl))
	requiredValT = template.Must(template.New("req").Funcs(fm).Parse(requiredValTmpl))
	arrayValT = template.Must(template.New("array").Funcs(fm).Parse(arrayValTmpl))
	mapValT = template.Must(template.New("map").Funcs(fm).Parse(mapValTmpl))
	userValT = template.Must(template.New("user").Funcs(fm).Parse(userValTmpl))
}

// HasValidations returns true if the given attribute or any of its children
// recursively has validations. If ignoreRequired is true then HasValidation
// does not consider "Required" validations. This is necessary to know whether
// validation code should be generated for types that don't use pointers to
// define required fields.
func HasValidations(att *design.AttributeExpr, ignoreRequired bool) bool {
	if att.Validation != nil {
		if !ignoreRequired || !att.Validation.HasRequiredOnly() {
			return true
		}
	}
	if o := design.AsObject(att.Type); o != nil {
		for _, catt := range o {
			seen := make(map[*design.AttributeExpr]struct{})
			seen[att] = struct{}{}
			if hasValidationsRecurse(catt, ignoreRequired, seen) {
				return true
			}
		}
	}
	return false
}

// ValidationCode produces Go code that runs the validations defined in the
// given attribute definition if any against the content of the variable named
// target. The generated code assumes that there is a pre-existing "err"
// variable of type error. It initializes that variable in case a validation
// fails.
//
// req indicates whether the attribute is required (true) or optional (false) in
// which case target is assumed to be a pointer.
//
// pub indicates whether the data structure described by att is public (true) or
// private (false). A private data structure uses pointers to hold all attributes
// so that they may be properly validated.
//
// context is used to produce helpful messages in case of error.
//
func ValidationCode(att *design.AttributeExpr, req, pub bool, target, context string) string {
	validation := att.Validation
	if validation == nil {
		return ""
	}
	var (
		kind            = att.Type.Kind()
		isNativePointer = kind == design.BytesKind || kind == design.AnyKind
		isPointer       = !pub || (!req && att.DefaultValue == nil)
		tval            = target
	)
	if isPointer && design.IsPrimitive(att.Type) && !isNativePointer {
		tval = "*" + tval
	}
	data := map[string]interface{}{
		"attribute": att,
		"isPointer": isPointer,
		"context":   context,
		"target":    target,
		"targetVal": tval,
		"string":    kind == design.StringKind,
		"array":     design.IsArray(att.Type),
		"map":       design.IsMap(att.Type),
		"pub":       pub,
	}
	runTemplate := func(tmpl *template.Template, data interface{}) string {
		var buf bytes.Buffer
		if err := tmpl.Execute(&buf, data); err != nil {
			panic(err) // bug

		}
		return buf.String()
	}
	var res []string
	if values := validation.Values; values != nil {
		data["values"] = values
		if val := runTemplate(enumValT, data); val != "" {
			res = append(res, val)
		}
	}
	if format := validation.Format; format != "" {
		data["format"] = format
		if val := runTemplate(formatValT, data); val != "" {
			res = append(res, val)
		}
	}
	if pattern := validation.Pattern; pattern != "" {
		data["pattern"] = pattern
		if val := runTemplate(patternValT, data); val != "" {
			res = append(res, val)
		}
	}
	if min := validation.Minimum; min != nil {
		data["min"] = *min
		data["isMin"] = true
		delete(data, "max")
		if val := runTemplate(minMaxValT, data); val != "" {
			res = append(res, val)
		}
	}
	if max := validation.Maximum; max != nil {
		data["max"] = *max
		data["isMin"] = false
		delete(data, "min")
		if val := runTemplate(minMaxValT, data); val != "" {
			res = append(res, val)
		}
	}
	if minLength := validation.MinLength; minLength != nil {
		data["minLength"] = minLength
		data["isMinLength"] = true
		delete(data, "maxLength")
		if val := runTemplate(lengthValT, data); val != "" {
			res = append(res, val)
		}
	}
	if maxLength := validation.MaxLength; maxLength != nil {
		data["maxLength"] = maxLength
		data["isMinLength"] = false
		delete(data, "minLength")
		if val := runTemplate(lengthValT, data); val != "" {
			res = append(res, val)
		}
	}
	if req := validation.Required; len(req) > 0 {
		for _, r := range req {
			reqAtt := design.AsObject(att.Type)[r]
			if reqAtt == nil {
				continue
			}
			if pub && design.IsPrimitive(reqAtt.Type) &&
				reqAtt.Type.Kind() != design.BytesKind &&
				reqAtt.Type.Kind() != design.AnyKind {

				continue
			}
			data["req"] = r
			data["reqAtt"] = reqAtt
			res = append(res, runTemplate(requiredValT, data))
		}
	}
	return strings.Join(res, "\n")
}

// RecursiveValidationCode produces Go code that runs the validations defined in
// the given attribute and its children recursively against the value held by
// the variable named target. See ValidationCode for a description of the
// arguments and their effects.
func RecursiveValidationCode(att *design.AttributeExpr, req, pub bool, target string) string {
	seen := make(map[string]*bytes.Buffer)
	return recurseValidationCode(att, req, pub, target, target, seen).String()
}

func hasValidationsRecurse(att *design.AttributeExpr, ignoreRequired bool, seen map[*design.AttributeExpr]struct{}) bool {
	if att.Validation != nil {
		if !ignoreRequired || !att.Validation.HasRequiredOnly() {
			return true
		}
	}
	if o := design.AsObject(att.Type); o != nil {
		for _, catt := range o {
			if _, ok := seen[catt]; ok {
				continue // break infinite recursions
			}
			seen[catt] = struct{}{}
			if hasValidationsRecurse(catt, ignoreRequired, seen) {
				return true
			}
		}
	}
	return false
}

func recurseValidationCode(att *design.AttributeExpr, req, pub bool, target, context string, seen map[string]*bytes.Buffer) *bytes.Buffer {
	var (
		buf   = new(bytes.Buffer)
		first = true
	)

	// Break infinite recursions
	if ut, ok := att.Type.(design.UserType); ok {
		if buf, ok := seen[ut.Name()]; ok {
			return buf
		}
		seen[ut.Name()] = buf
	}

	validation := ValidationCode(att, req, pub, target, context)
	if validation != "" {
		buf.WriteString(validation)
		first = false
	}

	if o := design.AsObject(att.Type); o != nil {
		WalkAttributes(o, func(n string, catt *design.AttributeExpr) error {
			validation := recurseAttribute(att, catt, n, target, context, pub, seen)
			if validation != "" {
				if !first {
					buf.WriteByte('\n')
				} else {
					first = false
				}
				buf.WriteString(validation)
			}
			return nil
		})
	} else if a := design.AsArray(att.Type); a != nil {
		val := recurseValidationCode(a.ElemType, true, true, "e", context+"[*]", seen).String()
		if val != "" {
			switch a.ElemType.Type.(type) {
			case design.UserType:
				// For user and result types, call the Validate method
				var buf bytes.Buffer
				if err := userValT.Execute(&buf, map[string]interface{}{"target": "e"}); err != nil {
					panic(err) // bug
				}
				val = fmt.Sprintf("if e != nil {\n\t%s\n}", buf.String())
			}
			data := map[string]interface{}{
				"target":     target,
				"validation": val,
			}
			if !first {
				buf.WriteByte('\n')
			} else {
				first = false
			}
			if err := arrayValT.Execute(buf, data); err != nil {
				panic(err) // bug
			}
		}
	} else if m := design.AsMap(att.Type); m != nil {
		keyVal := recurseValidationCode(m.KeyType, true, true, "k", context+".key", seen).String()
		valueVal := recurseValidationCode(m.ElemType, true, true, "v", context+"[key]", seen).String()
		if keyVal != "" || valueVal != "" {
			if keyVal != "" {
				if _, ok := m.KeyType.Type.(design.UserType); ok {
					var buf bytes.Buffer
					if err := userValT.Execute(&buf, map[string]interface{}{"target": "k"}); err != nil {
						panic(err) // bug
					}
					keyVal = fmt.Sprintf("\nif k != nil {\n\t%s\n}", buf.String())
				} else {
					keyVal = "\n" + keyVal
				}
			}
			if valueVal != "" {
				if _, ok := m.ElemType.Type.(design.UserType); ok {
					var buf bytes.Buffer
					if err := userValT.Execute(&buf, map[string]interface{}{"target": "v"}); err != nil {
						panic(err) // bug
					}
					valueVal = fmt.Sprintf("\nif v != nil {\n\t%s\n}", buf.String())
				} else {
					valueVal = "\n" + valueVal
				}
			}
			data := map[string]interface{}{
				"target":          target,
				"keyValidation":   keyVal,
				"valueValidation": valueVal,
			}
			if !first {
				buf.WriteByte('\n')
			} else {
				first = false
			}
			if err := mapValT.Execute(buf, data); err != nil {
				panic(err) // bug
			}
		}
	}
	return buf
}

func recurseAttribute(att, catt *design.AttributeExpr, n, target, context string, pub bool, seen map[string]*bytes.Buffer) string {
	var validation string
	if ut, ok := catt.Type.(design.UserType); ok {
		// We need to check empirically whether there are validations to be
		// generated, we can't just generate and check whether something was
		// generated to avoid infinite recursions.
		hasValidations := false
		done := errors.New("done")
		Walk(ut.Attribute(), func(a *design.AttributeExpr) error {
			if a.Validation != nil {
				if !pub {
					hasValidations = true
					return done
				}
				// For public data structures there is a case
				// where there is validation but no actual
				// validation code: if the validation is a
				// required validation that applies to
				// attributes that cannot be nil or empty string
				// i.e. primitive types other than string.
				if !a.Validation.HasRequiredOnly() {
					hasValidations = true
					return done
				}
				for _, name := range a.Validation.Required {
					att := design.AsObject(a.Type)[name]
					if att != nil && (!design.IsPrimitive(att.Type) || att.Type.Kind() == design.StringKind) {
						hasValidations = true
						return done
					}
				}
			}
			return nil
		})
		if hasValidations {
			var buf bytes.Buffer
			tgt := fmt.Sprintf("%s.%s", target, GoifyAtt(catt, n, true))
			if err := userValT.Execute(&buf, map[string]interface{}{"target": tgt}); err != nil {
				panic(err) // bug
			}
			validation = buf.String()
		}
	} else {
		validation = recurseValidationCode(
			catt,
			att.IsRequired(n),
			pub,
			fmt.Sprintf("%s.%s", target, GoifyAtt(catt, n, true)),
			fmt.Sprintf("%s.%s", context, n),
			seen,
		).String()
	}
	if validation != "" {
		if design.IsObject(catt.Type) {
			validation = fmt.Sprintf("if %s.%s != nil {\n%s\n}",
				target, GoifyAtt(catt, n, true), validation)
		}
	}
	return validation
}

// toSlice returns Go code that represents the given slice.
func toSlice(val []interface{}) string {
	elems := make([]string, len(val))
	for i, v := range val {
		elems[i] = fmt.Sprintf("%#v", v)
	}
	return fmt.Sprintf("[]interface{}{%s}", strings.Join(elems, ", "))
}

// oneof produces code that compares target with each element of vals and ORs
// the result, e.g. "target == 1 || target == 2".
func oneof(target string, vals []interface{}) string {
	elems := make([]string, len(vals))
	for i, v := range vals {
		elems[i] = fmt.Sprintf("%s == %#v", target, v)
	}
	return strings.Join(elems, " || ")
}

// constant returns the Go constant name of the format with the given value.
func constant(formatName string) string {
	switch formatName {
	case "date-time":
		return "goa.FormatDateTime"
	case "email":
		return "goa.FormatEmail"
	case "hostname":
		return "goa.FormatHostname"
	case "ipv4":
		return "goa.FormatIPv4"
	case "ipv6":
		return "goa.FormatIPv6"
	case "ip":
		return "goa.FormatIP"
	case "uri":
		return "goa.FormatURI"
	case "mac":
		return "goa.FormatMAC"
	case "cidr":
		return "goa.FormatCIDR"
	case "regexp":
		return "goa.FormatRegexp"
	}
	panic("unknown format") // bug
}

const (
	arrayValTmpl = `for _, e := range {{ .target }} {
{{ .validation }}
}`

	mapValTmpl = `for {{if .keyValidation }}k{{ else }}_{{ end }}, {{ if .valueValidation }}v{{ else }}_{{ end }} := range {{ .target }} {
{{- .keyValidation }}
{{- .valueValidation }}
}`

	userValTmpl = `if err2 := {{ .target }}.Validate(); err2 != nil {
	err = goa.MergeErrors(err, err2)
}`

	enumValTmpl = `{{ if .isPointer -}}
if {{ .target }} != nil {
{{ end -}}
if !({{ oneof .targetVal .values }}) {
	err = goa.MergeErrors(err, goa.InvalidEnumValueError({{ printf "%q" .context }}, {{ .targetVal }}, {{ slice .values }}))
{{ if .isPointer -}}
}
{{ end -}}
}`

	patternValTmpl = `{{ if .isPointer -}}
if {{ .target }} != nil {
{{ end -}}
	err = goa.MergeErrors(err, goa.ValidatePattern({{ printf "%q" .context }}, {{ .targetVal }}, {{ printf "%q" .pattern }}))
{{- if .isPointer }}
}
{{- end }}`

	formatValTmpl = `{{ if .isPointer -}}
if {{ .target }} != nil {
{{ end -}}
	err = goa.MergeErrors(err, goa.ValidateFormat({{ printf "%q" .context }}, {{ .targetVal}}, {{ constant .format }}))
{{ if .isPointer -}}
}
{{ end -}}
}`

	minMaxValTmpl = `{{ if .isPointer -}}
if {{ .target }} != nil {
{{ end -}}
	if {{ .targetVal }} {{ if .isMin }}<{{ else }}>{{ end }} {{ if .isMin }}{{ .min }}{{ else }}{{ .max }}{{ end }} {
	err = goa.MergeErrors(err, goa.InvalidRangeError({{ printf "%q" .context }}, {{ .targetVal }}, {{ if .isMin }}{{ .min }}, true{{ else }}{{ .max }}, false{{ end }}))
{{ if .isPointer -}}
}
{{ end -}}
}`

	lengthValTmpl = `{{ $target := or (and (or (or .array .map) .nonzero) .target) .targetVal -}}
{{- if .isPointer -}}
if {{ .target }} != nil {
{{ end -}}
	if {{ if .string }}utf8.RuneCountInString({{ $target }}){{ else }}len({{ $target }}){{ end }} {{ if .isMinLength }}<{{ else }}>{{ end }} {{ if .isMinLength }}{{ .minLength }}{{ else }}{{ .maxLength }}{{ end }} {
	err = goa.MergeErrors(err, goa.InvalidLengthError({{ printf "%q" .context }}, {{ $target }}, {{ if .string }}utf8.RuneCountInString({{ $target }}){{ else }}len({{ $target }}){{ end }}, {{ if .isMinLength }}{{ .minLength }}, true{{ else }}{{ .maxLength }}, false{{ end }}))
{{ if .isPointer -}}
	}
{{ end -}}
}`

	requiredValTmpl = `if {{ $.target }}.{{ goifyAtt $.reqAtt .req true }} == nil {
	err = goa.MergeErrors(err, goa.MissingAttributeError({{ printf "%q" $.context }}, "{{ .req }}"))
}`
)
