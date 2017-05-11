// TBD
// MarshalTypes codegen file
// Change RequestBodyType like RequestResponseType
// Write decode code
// use name scope for types? (hopefully don't have to)
// add tags to generated type from restgen.GoTypeDef
// Make sure to generate user type for each error even if declared with primitive
// Add metadata to specify error type attribute to be used for error message
// validate only one error per http status code
// Make sure primitive rename of []byte to bytes didn't break stuff

// DSL validation: make sure there's at least one response for all actions

package rest

import (
	"fmt"
	"net/http"
	"path/filepath"
	"text/template"

	"strings"

	"goa.design/goa.v2/codegen"
	restgen "goa.design/goa.v2/codegen/rest"
	"goa.design/goa.v2/design"
	"goa.design/goa.v2/design/rest"
)

type (
	// serverActionData describes a single endpoint/resource.
	serverData struct {
		// ServiceName is the name of the service.
		ServiceName string
		// VarServiceName is the goified name of the service.
		VarServiceName string
		// HandlerStruct is the name of the main server handler structure.
		HandlersStruct string
		// Constructor is the name of the constructor of the handler struct function.
		Constructor string
		// MountHandlers is the name of the name of the mount function.
		MountHandlers string
		// Action describes the action data for this endpoint.
		ActionData []*serverActionData
	}

	// serverActionData describes a single action.
	serverActionData struct {
		// EndpointName is the name of the endpoint.
		EndpointName string
		// VarEndpointName is the goified name of theendpoint/resource.
		VarEndpointName string
		// ServiceName is the name of the service.
		ServiceName string
		// VarServiceName is the goified name of the service.
		VarServiceName string
		// Routes describes the possible routes for this action.
		Routes []*serverRouteData
		// MountHandler is the name of the mount handler function.
		MountHandler string
		// Constructor is the name of the constructor function for the http handler function.
		Constructor string
		// Decoder is the name of the decoder function.
		Decoder string
		// Encoder is the name of the encoder function.
		Encoder string
		// ErrorEncoder is the name of the error encoder function.
		ErrorEncoder string
		// Payload provides information about the payload.
		Payload *serverPayloadData
		// ResultTypeName is the service endpoint result type name
		ResultTypeName string
		// Responses describes the information about the different
		// responses. If there are more than one responses then the
		// tagless response must be last.
		Responses []*serverResponseData
		// HTTPErrors describes the information about error responses.
		HTTPErrors []*serverErrorData
	}

	// serverPayloadData describes a payload.
	serverPayloadData struct {
		// Name is the name of the payload structure.
		Name string
		// Constructor is the name of the payload constructor function.
		Constructor string
		// BodyTypeName is the name of the request body type if any.
		BodyTypeName string
		// PathParams describes the information about params that are
		// present in the path.
		PathParams []*serverParamData
		// QueryParams describes the information about the params that
		// are present in the query.
		QueryParams []*serverParamData
		// AllParams describes the params, in path and query.
		AllParams []*serverParamData
		// ValidateBody contains the code used to validate the request
		// body if any.
		ValidateBody string
	}

	// serverRouteData describes a route.
	serverRouteData struct {
		// Method is the HTTP method.
		Method string
		// Path is the full path.
		Path string
	}

	// serverResponseData describes a response.
	serverResponseData struct {
		// Body is the type of the response body, nil if body should be
		// empty.
		Body design.DataType
		// BodyUserTypeName is the name of the Body type if it is a user
		// or media type, the empty string otherwise.
		BodyUserTypeName string
		// StatusCode is the return code of the response.
		StatusCode string
		// Headers provides information about the headers in the response.
		Headers []*serverHeaderData
		// TagName is the name of the attribute used to test whether the
		// response is the one to use.
		TagName string
		// TagValue is the value the result attribute named by TagName
		// must have for this response to be used.
		TagValue string
		// ResultToBody maps the result type attribute names to the
		// response body attribute names if the type differ (i.e. if
		// there are mapped attribute or if there are response headers)
		ResultToBody map[string]string
	}

	// serverErrorData describes a error response.
	serverErrorData struct {
		// TypeRef is a reference to the user type.
		TypeRef string
		// Response is the error response data.
		Response *serverResponseData
	}

	// serverParamData describes a parameter.
	serverParamData struct {
		// Name is the name of the mapping to the actual variable name.
		Name string
		// FieldName is the name of the struct field that holds the
		// param value.
		FieldName string
		// VarName is the name of the Go variable used to read or
		// convert the param value.
		VarName string
		// Required is true if the param is required.
		Required bool
		// Type is the datatype of the variable.
		Type design.DataType
		// Validate contains the validation code if any.
		Validate string
	}

	// serverHeaderData describes a header.
	serverHeaderData struct {
		// Name describes the name of the header key.
		Name string
		// FieldName is the name of the struct field that holds the
		// header value.
		FieldName string
		// VarName is the name of the Go variable used to read or
		// convert the header value.
		VarName string
		// Required is true if the header is required.
		Required bool
		// Type describes the datatype of the variable value. Mainly used for conversion.
		Type design.DataType
	}
)

var (
	serverTmpl = template.New("server").Funcs(template.FuncMap{
		"goTypeRef": codegen.GoTypeRef,
	}).Funcs(codegen.TemplateFuncs())
	serverStructTmpl             = template.Must(serverTmpl.New("struct").Parse(serverStructT))
	serverConstructorTmpl        = template.Must(serverTmpl.New("constructor").Parse(serverConstructorT))
	serverMountTmpl              = template.Must(serverTmpl.New("mount").Parse(serverMountT))
	serverHandlerTmpl            = template.Must(serverTmpl.New("handler").Parse(serverHandlerT))
	serverHandlerConstructorTmpl = template.Must(serverTmpl.New("handler_constructor").Parse(serverHandlerConstructorT))
	serverDecoderTmpl            = template.Must(serverTmpl.New("decoder").Parse(serverDecoderT))
	serverEncoderTmpl            = template.Must(serverTmpl.New("encoder").Parse(serverEncoderT))
	serverErrorEncoderTmpl       = template.Must(serverTmpl.New("error_encoder").Parse(serverErrorEncoderT))
)

// ServerFiles returns all the server HTTP transport files.
func ServerFiles(root *rest.RootExpr) []codegen.File {
	fw := make([]codegen.File, len(root.Resources))
	for i, r := range root.Resources {
		fw[i] = Server(r)
	}
	return fw
}

// Server returns the server HTTP transport file
func Server(r *rest.ResourceExpr) codegen.File {
	path := filepath.Join("transport", "http", codegen.SnakeCase(r.Name())+"_server.go")
	sections := func(genPkg string) []*codegen.Section {
		d := buildServerData(r)

		title := fmt.Sprintf("%s server HTTP transport", r.Name())
		s := []*codegen.Section{
			codegen.Header(title, "http", []*codegen.ImportSpec{
				{Path: "fmt"},
				{Path: "io"},
				{Path: "net/http"},
				{Path: "strconv"},
				{Path: "strings"},
				{Path: "goa.design/goa.v2"},
				{Path: "goa.design/goa.v2/rest"},
				{Path: genPkg + "/endpoints"},
				{Path: genPkg + "/services"},
			}),
			{Template: serverStructTmpl, Data: d},
			{Template: serverConstructorTmpl, Data: d},
			{Template: serverMountTmpl, Data: d},
		}

		for _, a := range d.ActionData {
			as := []*codegen.Section{
				{Template: serverHandlerTmpl, Data: a},
				{Template: serverHandlerConstructorTmpl, Data: a},
			}
			s = append(s, as...)

			if a.HasResponses() {
				s = append(s, &codegen.Section{Template: serverEncoderTmpl, Data: a})
			}

			if a.HasPayload() {
				s = append(s, &codegen.Section{Template: serverDecoderTmpl, Data: a})
			}

			if a.HasErrors() {
				s = append(s, &codegen.Section{Template: serverErrorEncoderTmpl, Data: a})
			}
		}
		return s
	}

	return codegen.NewSource(path, sections)
}

func buildServerData(r *rest.ResourceExpr) *serverData {
	varServiceName := codegen.Goify(r.Name(), true)
	sd := &serverData{
		ServiceName:    r.Name(),
		VarServiceName: varServiceName,
		HandlersStruct: fmt.Sprintf("%sHandlers", varServiceName),
		Constructor:    fmt.Sprintf("New%sHandlers", varServiceName),
		MountHandlers:  fmt.Sprintf("Mount%sHandlers", varServiceName),
	}

	for _, a := range r.Actions {
		varEndpointName := codegen.Goify(a.Name(), true)
		var resultTypeName string
		if a.EndpointExpr.Result.Type != nil && a.EndpointExpr.Result.Type != design.Empty {
			resultTypeName = codegen.GoTypeName(a.EndpointExpr.Result.Type)
		}

		routes := make([]*serverRouteData, len(a.Routes))
		for i, v := range a.Routes {
			routes[i] = &serverRouteData{
				Method: strings.ToUpper(v.Method),
				Path:   v.FullPath(),
			}
		}

		var responses []*serverResponseData
		notag := -1
		for i, v := range a.Responses {
			if v.Tag[0] == "" {
				if notag > -1 {
					continue // we don't want more than one response with no tag
				}
				notag = i
			}
			responses = append(responses, buildResponseData(r, a, v))
		}
		count := len(responses)
		if notag >= 0 && notag < count-1 {
			// Make sure tagless response is last
			responses[notag], responses[count-1] = responses[count-1], responses[notag]
		}

		httperrs := make([]*serverErrorData, len(a.HTTPErrors))
		for i, v := range a.HTTPErrors {
			httperrs[i] = buildErrorData(r, a, v)
		}

		ad := &serverActionData{
			EndpointName:    a.Name(),
			VarEndpointName: varEndpointName,
			ServiceName:     r.Name(),
			VarServiceName:  varServiceName,
			Routes:          routes,
			ResultTypeName:  resultTypeName,
			Responses:       responses,
			HTTPErrors:      httperrs,
			MountHandler:    fmt.Sprintf("Mount%s%sHandler", varEndpointName, varServiceName),
			Constructor:     fmt.Sprintf("New%s%sHandler", varEndpointName, varServiceName),
			Decoder:         fmt.Sprintf("%s%sDecodeRequest", varEndpointName, varServiceName),
			Encoder:         fmt.Sprintf("%s%sEncodeResponse", varEndpointName, varServiceName),
			ErrorEncoder:    fmt.Sprintf("%s%sEncodeError", varEndpointName, varServiceName),
		}

		if a.EndpointExpr.Payload.Type != design.Empty {
			name := codegen.Goify(r.Name(), false) + codegen.Goify(a.Name(), true) + "RequestBody"
			body, public := restgen.RequestBodyType(a, name)
			var (
				validate     string
				bodyTypeName string
			)
			{
				if ut, ok := body.(design.UserType); ok {
					validate = codegen.RecursiveValidationCode(ut.Attribute(), true, true, "body", "body")
				}
				bodyTypeName = codegen.GoTypeName(body)
				if public {
					bodyTypeName = "service." + bodyTypeName
				}
			}
			ad.Payload = &serverPayloadData{
				Name:         fmt.Sprintf("%s%sPayload", varEndpointName, varServiceName),
				Constructor:  fmt.Sprintf("New%s%sPayload", varEndpointName, varServiceName),
				BodyTypeName: bodyTypeName,
				PathParams:   extractParams(a.PathParams()),
				QueryParams:  extractParams(a.QueryParams()),
				AllParams:    extractParams(a.AllParams()),
				ValidateBody: validate,
			}
		}
		sd.ActionData = append(sd.ActionData, ad)
	}
	return sd
}

func buildResponseData(r *rest.ResourceExpr, a *rest.ActionExpr, v *rest.HTTPResponseExpr) *serverResponseData {
	var bodyTypeName string
	name := codegen.Goify(r.Name(), true) + codegen.Goify(a.Name(), true)
	if len(a.Responses) > 1 {
		name += http.StatusText(v.StatusCode)
	}
	name += "ResponseBody"
	body := restgen.ResponseBodyType(a.EndpointExpr.Result, v, name)
	if body != nil {
		if ut, ok := body.(design.UserType); ok {
			bodyTypeName = ut.Name()
		}
	}
	var resultToBody map[string]string
	if design.IsObject(a.EndpointExpr.Result.Type) && design.IsObject(body) {
		rb, mapped := serviceTypeToBody(a.EndpointExpr.Result.Type, body)
		if body != a.EndpointExpr.Result.Type || mapped {
			resultToBody = rb
		}
	}
	return &serverResponseData{
		Body:             body,
		BodyUserTypeName: bodyTypeName,
		StatusCode:       restgen.StatusCodeToHTTPConst(v.StatusCode),
		Headers:          extractHeaders(v.MappedHeaders()),
		TagName:          v.Tag[0],
		TagValue:         v.Tag[1],
		ResultToBody:     resultToBody,
	}
}

func buildErrorData(r *rest.ResourceExpr, a *rest.ActionExpr, v *rest.HTTPErrorExpr) *serverErrorData {
	var bodyTypeName string
	name := codegen.Goify(r.Name(), true) +
		codegen.Goify(a.Name(), true) +
		http.StatusText(v.Response.StatusCode) +
		"Response"
	body := restgen.ResponseBodyType(v.ErrorExpr.AttributeExpr, v.Response, name)
	if body != nil {
		if ut, ok := body.(design.UserType); ok {
			bodyTypeName = ut.Name()
		}
	}
	var resultToBody map[string]string
	mapped := false
	if design.IsObject(body) {
		resultToBody = make(map[string]string)
		matt := rest.NewMappedAttributeExpr(&design.AttributeExpr{Type: body})
		obj := design.AsObject(v.ErrorExpr.Type)
		restgen.WalkMappedAttr(matt, func(name, elem string, required bool, a *design.AttributeExpr) error {
			if _, ok := obj[name]; ok {
				resultToBody[name] = elem
				if name != elem {
					mapped = true
				}
			}
			return nil
		})
	}
	if body == v.ErrorExpr.Type && !mapped {
		resultToBody = nil // no need to generate mapping
	}
	response := serverResponseData{
		Body:             body,
		BodyUserTypeName: bodyTypeName,
		StatusCode:       restgen.StatusCodeToHTTPConst(v.Response.StatusCode),
		Headers:          extractHeaders(v.Response.MappedHeaders()),
		ResultToBody:     resultToBody,
	}
	return &serverErrorData{
		TypeRef:  codegen.GoPackageTypeRef(v.ErrorExpr.Type, "service"),
		Response: &response,
	}
}

func extractHeaders(a *rest.MappedAttributeExpr) []*serverHeaderData {
	var headers []*serverHeaderData
	restgen.WalkMappedAttr(a, func(name, elem string, required bool, c *design.AttributeExpr) error {
		headers = append(headers, &serverHeaderData{
			Name:      elem,
			FieldName: codegen.Goify(name, true),
			VarName:   codegen.Goify(name, false),
			Required:  required,
			Type:      c.Type,
		})
		return nil
	})

	return headers
}

func extractParams(a *rest.MappedAttributeExpr) []*serverParamData {
	var params []*serverParamData
	restgen.WalkMappedAttr(a, func(name, elem string, required bool, c *design.AttributeExpr) error {
		field := codegen.Goify(name, true)
		varn := codegen.Goify(name, false)
		params = append(params, &serverParamData{
			Name:      elem,
			FieldName: field,
			VarName:   varn,
			Required:  required,
			Type:      c.Type,
			Validate:  codegen.ValidationCode(c, a.IsRequired(name), false, varn, varn),
		})
		return nil
	})

	return params
}

// serviceTypeToBody takes a endpoint payload or result type and the
// corresponding HTTP request or response body type and builds a mapping of
// endpoint type attribute name to body type attribute name. The names may
// differ if the DSL uses the "a:b" notation. In that case the second return
// value is true. Also some body attributes may not have equivalents in endpoint
// type if the body was explicitly defined in the DSL. In this case the
// generated code won't automatically initialized non-matching attributes, the
// user may want to augment the generated code with custom code.
func serviceTypeToBody(dt, body design.DataType) (map[string]string, bool) {
	mapped := false
	toBody := make(map[string]string)
	obj := design.AsObject(dt)
	matt := rest.NewMappedAttributeExpr(&design.AttributeExpr{Type: body})
	restgen.WalkMappedAttr(matt, func(name, elem string, required bool, a *design.AttributeExpr) error {
		if _, ok := obj[name]; ok {
			toBody[name] = elem
			if name != elem {
				mapped = true
			}
		}
		return nil
	})
	return toBody, mapped
}

// HasResponses indicates if an action has responses.
func (d *serverActionData) HasResponses() bool {
	return len(d.Responses) >= 1
}

// HasPayload indicates if an action has a payload.
func (d *serverActionData) HasPayload() bool {
	return d.Payload != nil
}

// HasErrors indicates if an action has errors defined.
func (d *serverActionData) HasErrors() bool {
	return len(d.HTTPErrors) > 0
}

// HasPathParams indicates if a payload has path parameters.
func (d *serverPayloadData) HasPathParams() bool {
	return len(d.PathParams) > 0
}

// HasQueryParams indicates if a payload has query parameters.
func (d *serverPayloadData) HasQueryParams() bool {
	return len(d.QueryParams) > 0
}

// HasParams indicates if a payload has any parameters.
func (d *serverPayloadData) HasParams() bool {
	return d.HasPathParams() || d.HasQueryParams()
}

const serverStructT = `{{ printf "%s lists the %s service endpoint HTTP handlers." .HandlersStruct .ServiceName | comment }}
type {{ .HandlersStruct }} struct {
	{{- range .ActionData }}
	{{ .EndpointName }} http.Handler
	{{- end }}
}
`

const serverConstructorT = `{{ printf "%s instantiates HTTP handlers for all the %s service endpoints." .Constructor .ServiceName | comment }}
func {{ .Constructor }}(
	e *endpoints.{{ .VarServiceName }},
	dec func(*http.Request) rest.Decoder,
	enc func(http.ResponseWriter, *http.Request) rest.Encoder,
	logger goa.LogAdapter,
) *{{ .HandlersStruct }} {
	return &{{ .HandlersStruct }}{
		{{- range .ActionData }}
		{{ .VarEndpointName }}: {{ .Constructor }}(e.{{ .VarEndpointName }}, dec, enc, logger),
		{{- end }}
	}
}
`

const serverMountT = `{{ printf "%s configures the mux to serve the %s endpoints." .MountHandlers .ServiceName | comment }}
func {{ .MountHandlers }}(mux rest.Muxer, h *{{ .HandlersStruct }}) {
	{{- range .ActionData }}
	{{ .MountHandler }}(mux, h.{{ .VarEndpointName }})
	{{- end }}
}
`

const serverHandlerT = `{{ printf "%s configures the mux to serve the \"%s\" service \"%s\" endpoint." .MountHandler .ServiceName .EndpointName | comment }}
func {{ .MountHandler }}(mux rest.Muxer, h http.Handler) {
	f, ok := h.(http.HandlerFunc)
	if !ok {
		f = func(w http.ResponseWriter, r *http.Request) {
			h.ServeHTTP(w, r)
		}
	}
	{{- range .Routes }}
	mux.Handle("{{ .Method }}", "{{ .Path }}", f)
	{{- end }}
}
`

const serverHandlerConstructorT = `{{ printf "%s creates a HTTP handler which loads the HTTP request and calls the \"%s\" service \"%s\" endpoint." .Constructor .ServiceName .EndpointName | comment }}
{{ comment "The middleware is mounted so it executes after the request is loaded and thus may access the request state via the rest package ContextXXX functions."}}
func {{ .Constructor }}(
	endpoint goa.Endpoint,
	dec func(*http.Request) rest.Decoder,
	enc func(http.ResponseWriter, *http.Request) rest.Encoder,
	logger goa.LogAdapter,
) http.Handler {
	var (
		{{- if .HasPayload }}
		decodeRequest  = {{ .Decoder }}(dec)
		{{- end }}
		{{- if .HasResponses }}
		encodeResponse = {{ .Encoder }}EncodeResponse(enc)
		{{- end }}
		encodeError    = {{ if .HasErrors }}{{ .ErrorEncoder }}{{ else }}rest.EncodeError{{ end }}(enc, logger)
	)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		{{- if .HasPayload }}
		payload, err := decodeRequest(r)
		if err != nil {
			encodeError(w, r, goa.ErrInvalid("request invalid: %s", err))
			return
		}

		res, err := endpoint(r.Context(), payload)
		{{- else }}
		res, err := endpoint(r.Context())
		{{- end }}

		if err != nil {
			encodeError(w, r, err)
			return
		}
		{{- if .HasResponses }}
		if err := encodeResponse(w, r, res); err != nil {
			encodeError(w, r, err)
		}
		{{- else }}
		w.Write(http.StatusNoContent)
		{{- end }}
	})
}
`

const serverDecoderT = `{{ printf "%s returns a decoder for requests sent to the create %s endpoint." .Decoder .ServiceName | comment }}
func {{ .Decoder }}(decoder func(*http.Request) rest.Decoder) func(*http.Request) (interface{}, error) {
	return func(r *http.Request) (*service.{{ .Payload.Name }}, error) {

{{- if .Payload.BodyTypeName }}
		var (
			body {{ .Payload.BodyTypeName }}
			err  error
		)
		err = decoder(r).Decode(&body)
		if err != nil {
			if err == io.EOF {
				err = goa.MissingPayloadError()
			}
			return nil, err
		}
		{{- .Payload.ValidateBody }}
{{ end }}

{{- if .Payload.HasParams }}
		{{- if .Payload.HasPathParams }}
		params := rest.ContextParams(r.Context())
		{{- end }}
		var (
			{{- range .Payload.AllParams }}
			{{ .VarName }} {{goTypeRef .Type false }}
			{{- end }}
		)

{{- range .Payload.QueryParams }}
	{{ .VarName }}{{ if not (eq .Type.Name "string" )}}Raw :{{ end }}= r.URL.Query().Get("{{ .Name }}")
	{{- if .Required }}if {{ .VarName }}{{ if not (eq .Type.Name "string" )}}Raw{{ end }} == "" {
		return nil, goa.MissingFieldError("{{ .Name }}", "query string")
	}{{ end }}
	{{- if not (eq .Type.Name "string") }}
		{{- template "conversion" . }}
	{{- end }}
	{{- .Validate }}
{{ end }}

{{- range .Payload.PathParams }}
	{{ .VarName }}{{ if not (eq .Type.Name "string" )}}Raw :{{ end }}= params["{{ .Name }}"]
	{{- if .Required }}if {{ .VarName }}{{ if not (eq .Type.Name "string" )}}Raw{{ end }} == "" {
		return nil, goa.MissingFieldError("{{ .Name }}", "path parameter")
	}{{ end }}
	{{- if not (eq .Type.Name "string") }}
		{{- template "conversion" . }}
	{{- end }}
	{{- .Validate }}
{{ end }}

{{- end }}
		return {{ .Payload.Constructor }}(
			{{- if .Payload.BodyTypeName }}&body{{ end -}}
			{{- range $i, $p := .Payload.AllParams }}
				{{- if or (ne $i 0) ($.Payload.BodyTypeName) }}, {{ end -}}{{ .VarName }}
			{{- end }})
	}
}
{{- define "conversion" }}
	{{- if eq .Type.Name "array" }}
		{{ .VarName }}RawSlice := strings.Split({{ .VarName }}Raw, ",")
		{{ .VarName }} = make({{ goTypeRef .Type false }}, len({{ .VarName }}RawSlice))
		for i, rv := range {{ .VarName }}RawSlice {
			{{- template "type_slice_conversion" . }}
		}
	{{- else }}
		{{- template "type_conversion" . }}
	{{- end }}
{{- end }}
{{- define "type_conversion" }}
	{{- if eq .Type.Name "string" }}
		{{ .VarName }} = url.QueryUnescape(v)
	{{- else if eq .Type.Name "[]byte" }}
		{{ .VarName }} = url.QueryUnescape(string(v))
	{{- else if eq .Type.Name "int" }}
		v, err := strconv.ParseInt({{ .VarName }}Raw, 10, strconv.IntSize)
		if err != nil {
			return nil, goa.InvalidFieldTypeError({{ .VarName}}Raw, {{ .Name }}, "integer")
		}
		{{ .VarName }} = int(v)
	{{- else if eq .Type.Name "int32" }}
		v, err := strconv.ParseInt({{ .VarName }}Raw, 10, 32)
		if err != nil {
			return nil, goa.InvalidFieldTypeError({{ .VarName}}Raw, {{ .Name }}, "integer")
		}
		{{ .VarName }} = int32(v)
	{{- else if eq .Type.Name "int64" }}
		v, err := strconv.ParseInt({{ .VarName }}Raw, 10, 64)
		if err != nil {
			return nil, goa.InvalidFieldTypeError({{ .VarName}}Raw, {{ .Name }}, "integer")
		}
		{{ .VarName }} = v
	{{- else if eq .Type.Name "uint" }}
		v, err := strconv.ParseUint({{ .VarName }}Raw, 10, strconv.IntSize)
		if err != nil {
			return nil, goa.InvalidFieldTypeError({{ .VarName}}Raw, {{ .Name }}, "unsigned integer")
		}
		{{ .VarName }} = uint(v)
	{{- else if eq .Type.Name "uint32" }}
		v, err := strconv.ParseUint({{ .VarName }}Raw, 10, 32)
		if err != nil {
			return nil, goa.InvalidFieldTypeError({{ .VarName}}Raw, {{ .Name }}, "unsigned integer")
		}
		{{ .VarName }} = int32(v)
	{{- else if eq .Type.Name "uint64" }}
		v, err := strconv.ParseUint({{ .VarName }}Raw, 10, 64)
		if err != nil {
			return nil, goa.InvalidFieldTypeError({{ .VarName}}Raw, {{ .Name }}, "unsigned integer")
		}
		{{ .VarName }} = v
	{{- else if eq .Type.Name "float32" }}
		v, err := strconv.ParseFloat({{ .VarName }}Raw, 32)
		if err != nil {
			return nil, goa.InvalidFieldTypeError({{ .VarName}}Raw, {{ .Name }}, "float")
		}
		{{ .VarName }} = float32(v)
	{{- else if eq .Type.Name "float64" }}
		v, err := strconv.ParseFloat({{ .VarName }}Raw, 64)
		if err != nil {
			return nil, goa.InvalidFieldTypeError({{ .VarName}}Raw, {{ .Name }}, "float")
		}
		{{ .VarName }} = v
	{{- else if eq .Type.Name "boolean" }}
		v, err := strconv.ParseBool({{ .VarName }}Raw)
		if err != nil {
			return nil, goa.InvalidFieldTypeError({{ .VarName}}Raw, {{ .Name }}, "boolean")
		}
		{{ .VarName }} = v
	{{- else }}
		// unsupported type {{ .Type.Name }} for var {{ .VarName }}
	{{- end }}
{{- end }}
{{- define "type_slice_conversion" }}
		{{- if eq .Type.ElemType.Type.Name "string" }}
			{{ .VarName }}[i] = url.QueryUnescape(rv)
		{{- else if eq .Type.ElemType.Type.Name "[]byte" }}
			{{ .VarName }}[i] = url.QueryUnescape(string(rv))
		{{- else if eq .Type.ElemType.Type.Name "int" }}
			v, err := strconv.ParseInt(rv, 10, strconv.IntSize)
			if err != nil {
				return nil, goa.InvalidFieldTypeError({{ .VarName}}Raw, {{ .Name }}, "array of integers")
			}
			{{ .VarName }}[i] = int(v)
		{{- else if eq .Type.ElemType.Type.Name "int32" }}
			v, err := strconv.ParseInt(rv, 10, 32)
			if err != nil {
				return nil, goa.InvalidFieldTypeError({{ .VarName}}Raw, {{ .Name }}, "array of integers")
			}
			{{ .VarName }}[i] = int32(v)
		{{- else if eq .Type.ElemType.Type.Name "int64" }}
			v, err := strconv.ParseInt(rv, 10, 64)
			if err != nil {
				return nil, goa.InvalidFieldTypeError({{ .VarName}}Raw, {{ .Name }}, "array of integers")
			}
			{{ .VarName }}[i] = v
		{{- else if eq .Type.ElemType.Type.Name "uint" }}
			v, err := strconv.ParseUint(rv, 10, strconv.IntSize)
			if err != nil {
				return nil, goa.InvalidFieldTypeError({{ .VarName}}Raw, {{ .Name }}, "array of unsigned integers")
			}
			{{ .VarName }}[i] = uint(v)
		{{- else if eq .Type.ElemType.Type.Name "uint32" }}
			v, err := strconv.ParseUint(rv, 10, 32)
			if err != nil {
				return nil, goa.InvalidFieldTypeError({{ .VarName}}Raw, {{ .Name }}, "array of unsigned integers")
			}
			{{ .VarName }}[i] = int32(v)
		{{- else if eq .Type.ElemType.Type.Name "uint64" }}
			v, err := strconv.ParseUint(rv, 10, 64)
			if err != nil {
				return nil, goa.InvalidFieldTypeError({{ .VarName}}Raw, {{ .Name }}, "array of unsigned integers")
			}
			{{ .VarName }}[i] = v
		{{- else if eq .Type.ElemType.Type.Name "float32" }}
			v, err := strconv.ParseFloat(rv, 32)
			if err != nil {
				return nil, goa.InvalidFieldTypeError({{ .VarName}}Raw, {{ .Name }}, "array of floats")
			}
			{{ .VarName }}[i] = float32(v)
		{{- else if eq .Type.ElemType.Type.Name "float64" }}
			v, err := strconv.ParseFloat(rv, 64)
			if err != nil {
				return nil, goa.InvalidFieldTypeError({{ .VarName}}Raw, {{ .Name }}, "array of floats")
			}
			{{ .VarName }}[i] = v
		{{- else if eq .Type.ElemType.Type.Name "boolean" }}
			v, err := strconv.ParseBool(rv)
			if err != nil {
				return nil, goa.InvalidFieldTypeError({{ .VarName}}Raw, {{ .Name }}, "array of booleans")
			}
			{{ .VarName }}[i] = v
		{{- else }}
			// unsupported slice type {{ .Type.ElemType.Type.Name }} for var {{ .VarName }}
		{{- end }}
{{- end }}
`

const serverEncoderT = `{{ printf "%s returns an encoder for responses returned by the %s %s endpoint." .Encoder .EndpointName .ServiceName | comment }}
func {{ .Encoder }}(encoder func(http.ResponseWriter, *http.Request) rest.Encoder) func(http.ResponseWriter, *http.Request, interface{}) error {
	return func(w http.ResponseWriter, r *http.Request, v interface{}) error {

	{{- if .ResultTypeName }}
		t := v.(*service.{{ .ResultTypeName }})

		{{- range .Responses }}

			{{- if .TagName }}
		if t.{{ .TagName }} == {{ printf "%q" .TagValue }} {
			{{- end }}
			{{ template "response" . }}

			{{- if .ResultToBody }}	
			return enc.Encode(&body)
			{{- else }}	
			return enc.Encode(t)
			{{- end }}	

			{{- if .TagName }}
		}
			{{- end }}

		{{- end }}

	{{- else }}

		{{- with (index .Responses 0) }}

		{{- if .Body }}
		enc, ct := encoder(w, r)
		rest.SetContentType(w, ct)
		w.WriteHeader({{ .StatusCode }})
		return enc.Encode(v)

		{{- else }}
		w.WriteHeader({{ .StatusCode }})
		return nil

		{{- end }}

		{{- end }}

	{{- end }}
	}
}
` + responseT

const serverErrorEncoderT = `{{ printf "%s returns an encoder for errors returned by the %s %s endpoint." .ErrorEncoder .EndpointName .ServiceName | comment }}
func {{ .ErrorEncoder }}(encoder func(http.ResponseWriter, *http.Request) rest.Encoder, logger goa.LogAdapter) func(http.ResponseWriter, *http.Request, error) {
	encodeError := rest.EncodeError(encoder, logger)
	return func(w http.ResponseWriter, r *http.Request, v error) {
		switch t := v.(type) {

		{{- range .HTTPErrors }}
		case {{ .TypeRef }}:

			{{- template "response" .Response }}
			{{- if .Response.ResultToBody }}	
			if err := enc.Encode(&body); err != nil {
			{{- else }}	
			if err := enc.Encode(t); err != nil {
			{{- end }}
				encodeError(w, r, err)
			}

		{{- end }}
		default:
			encodeError(w, r, v)
		}
	}
}
` + responseT

const responseT = `{{ define "response" -}}
	enc, ct := encoder(w, r)
	rest.SetContentType(w, ct)

	{{- range .Headers }}
		{{- if not .Required }}
	if t.{{ .FieldName }} != nil {
		{{- end }}

		{{- if eq .Type.Name "string" }}
	w.Header().Set("{{ .Name }}", {{ if not .Required }}*{{ end }}t.{{ .FieldName }})
		{{- else }}
	v := t.{{ .FieldName }}
	{{ template "header_conversion" . }}
	w.Header().Set("{{ .Name }}", {{ .VarName }})
		{{- end }}

		{{- if not .Required }}
	}
		{{- end }}
	{{- end }}
	w.WriteHeader({{ .StatusCode }})

	{{- if .ResultToBody }}	
	body := {{ .BodyUserTypeName }}{
		{{- range $res, $bod := .ResultToBody }}
		{{ $bod }}: t.{{ $res }},
		{{- end }}
	}

	{{- end }}
{{- end }}

{{- define "header_conversion" }}
	{{- if eq .Type.Name "boolean" }}
		{{ .VarName }} := strconv.FormatBool({{ if not .Required }}*{{ end }}v)
	{{- else if eq .Type.Name "int" }}
		{{ .VarName }} := strconv.Itoa({{ if not .Required }}*{{ end }}v)
	{{- else if eq .Type.Name "int32" }}
		{{ .VarName }} := strconv.FormatInt(int64({{ if not .Required }}*{{ end }}v), 10)
	{{- else if eq .Type.Name "int64" }}
		{{ .VarName }} := strconv.FormatInt({{ if not .Required }}*{{ end }}v, 10)
	{{- else if eq .Type.Name "uint" }}
		{{ .VarName }} := strconv.FormatUint(uint64({{ if not .Required }}*{{ end }}v), 10)
	{{- else if eq .Type.Name "uint32" }}
		{{ .VarName }} := strconv.FormatUint(uint64({{ if not .Required }}*{{ end }}v), 10)
	{{- else if eq .Type.Name "uint64" }}
		{{ .VarName }} := strconv.FormatUint({{ if not .Required }}*{{ end }}v, 10)
	{{- else if eq .Type.Name "float32" }}
		{{ .VarName }} := strconv.FormatFloat(float64({{ if not .Required }}*{{ end }}v), 'f', -1, 32)
	{{- else if eq .Type.Name "float64" }}
		{{ .VarName }} := strconv.FormatFloat({{ if not .Required }}*{{ end }}v, 'f', -1, 64)
	{{- else if eq .Type.Name "string" }}
		{{ .VarName }} := v
	{{- else if eq .Type.Name "[]byte" }}
		{{ .VarName }} := string({{ if not .Required }}*{{ end }}v)
	{{- else }}
		// unsupported type {{ .Type.Name }} for header field {{ .FieldName }}
	{{- end }}
{{- end -}}
`
