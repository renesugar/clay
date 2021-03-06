package genhandler

import (
	"bytes"
	"strings"
	"text/template"

	pbdescriptor "github.com/golang/protobuf/protoc-gen-go/descriptor"
	"github.com/golang/protobuf/protoc-gen-go/generator"
	"github.com/grpc-ecosystem/grpc-gateway/protoc-gen-grpc-gateway/descriptor"
	"github.com/pkg/errors"
)

var (
	errNoTargetService = errors.New("no target service defined in the file")
)

var pkg map[string]string

type param struct {
	*descriptor.File
	Imports       []descriptor.GoPackage
	SwaggerBuffer []byte
}

func applyImplTemplate(p param) (string, error) {
	w := bytes.NewBuffer(nil)

	if err := implTemplate.Execute(w, p); err != nil {
		return "", err
	}

	return w.String(), nil
}

func applyDescTemplate(p param) (string, error) {
	// r := &http.Request{}
	// r.URL.Query()
	w := bytes.NewBuffer(nil)
	if err := headerTemplate.Execute(w, p); err != nil {
		return "", err
	}

	if err := regTemplate.ExecuteTemplate(w, "base", p); err != nil {
		return "", err
	}

	if p.SwaggerBuffer != nil {
		if err := footerTemplate.Execute(w, p); err != nil {
			return "", err
		}
	}
	if err := clientTemplate.Execute(w, p); err != nil {
		return "", err
	}

	if err := patternsTemplate.ExecuteTemplate(w, "base", p); err != nil {
		return "", err
	}

	return w.String(), nil
}

var (
	varNameReplacer = strings.NewReplacer(
		".", "_",
		"/", "_",
		"-", "_",
	)
	funcMap = template.FuncMap{
		"hasAsterisk": func(ss []string) bool {
			for _, s := range ss {
				if s == "*" {
					return true
				}
			}
			return false
		},
		"varName": func(s string) string { return varNameReplacer.Replace(s) },
		"goTypeName": func(s string) string {
			toks := strings.Split(s, ".")
			for pos := range toks {
				toks[pos] = generator.CamelCase(toks[pos])
			}
			return strings.Join(toks, ".")
		},
		"byteStr":         func(b []byte) string { return string(b) },
		"escapeBackTicks": func(s string) string { return strings.Replace(s, "`", "` + \"``\" + `", -1) },
		"toGoType":        func(t pbdescriptor.FieldDescriptorProto_Type) string { return primitiveTypeToGo(t) },
		// arrayToPathInterp replaces chi-style path to fmt.Sprint-style path.
		"arrayToPathInterp": func(tpl string) string {
			vv := strings.Split(tpl, "/")
			ret := []string{}
			for _, v := range vv {
				if strings.HasPrefix(v, "{") {
					ret = append(ret, "%v")
					continue
				}
				ret = append(ret, v)
			}
			return strings.Join(ret, "/")
		},
		// returns safe package prefix with dot(.) or empty string by imported package name or alias
		"pkg": func(name string) string {
			if p, ok := pkg[name]; ok && p != "" {
				return p + "."
			}
			return ""
		},
	}

	headerTemplate = template.Must(template.New("header").Funcs(funcMap).Parse(`
// Code generated by protoc-gen-goclay
// source: {{ .GetName }}
// DO NOT EDIT!

/*
Package {{ .GoPkg.Name }} is a self-registering gRPC and JSON+Swagger service definition.

It conforms to the github.com/utrack/clay Service interface.
*/
package {{ .GoPkg.Name }}
import (
    {{ range $i := .Imports }}{{ if $i.Standard }}{{ $i | printf "%s\n" }}{{ end }}{{ end }}

    {{ range $i := .Imports }}{{ if not $i.Standard }}{{ $i | printf "%s\n" }}{{ end }}{{ end }}
)

// Update your shared lib or downgrade generator to v1 if there's an error
var _ = {{ pkg "transport" }}IsVersion2

var _ {{ pkg "chi" }}Router
var _ {{ pkg "runtime" }}Marshaler
`))
	regTemplate = template.Must(template.New("svc-reg").Funcs(funcMap).Parse(`
{{ define "base" }}
{{ range $svc := .Services }}
// {{ $svc.GetName }}Desc is a descriptor/registrator for the {{ $svc.GetName }}Server.
type {{ $svc.GetName }}Desc struct {
      svc {{ $svc.GetName }}Server
}

// New{{ $svc.GetName }}ServiceDesc creates new registrator for the {{ $svc.GetName }}Server.
func New{{ $svc.GetName }}ServiceDesc(svc {{ $svc.GetName }}Server) *{{ $svc.GetName }}Desc {
      return &{{ $svc.GetName }}Desc{svc:svc}
}

// RegisterGRPC implements service registrator interface.
func (d *{{ $svc.GetName }}Desc) RegisterGRPC(s *{{ pkg "grpc" }}Server) {
      Register{{ $svc.GetName }}Server(s,d.svc)
}

// SwaggerDef returns this file's Swagger definition.
func (d *{{ $svc.GetName }}Desc) SwaggerDef(options ...{{ pkg "swagger" }}Option) (result []byte) {
    {{ if $.SwaggerBuffer }}if len(options) > 0 {
        var err error
        var swagger = &{{ pkg "spec" }}Swagger{}
        if err = {{ pkg "swagger" }}UnmarshalJSON(_swaggerDef_{{ varName $.GetName }}); err != nil {
            panic("Bad swagger definition: " + err.Error())
        }
        for _, o := range options {
            o(swagger)
        }
        if result, err = {{ pkg "swagger" }}MarshalJSON(); err != nil {
            panic("Failed marshal {{ pkg "spec" }}Swagger definition: " + err.Error())
        }
    } else {
        result = _swaggerDef_{{ varName $.GetName }}
    }
    {{ end -}}
    return result
}

// RegisterHTTP registers this service's HTTP handlers/bindings.
func (d *{{ $svc.GetName }}Desc) RegisterHTTP(mux {{ pkg "transport" }}Router) {
    chiMux, isChi := mux.({{ pkg "chi" }}Router)
    var h {{ pkg "http" }}HandlerFunc
    {{ range $m := $svc.Methods }}
    {{ range $b := $m.Bindings -}}
    // Handler for {{ $m.GetName }}, binding: {{ $b.HTTPMethod }} {{ $b.PathTmpl.Template }}
    h = {{ pkg "http" }}HandlerFunc(func(w {{ pkg "http" }}ResponseWriter, r *{{ pkg "http" }}Request) {
        defer r.Body.Close()

        var req {{ $m.RequestType.GetName }}
        err := unmarshaler_goclay_{{ $svc.GetName }}_{{ $m.GetName }}_{{ $b.Index }}(r,&req)
        if err != nil {
            {{ pkg "httpruntime" }}SetError(r.Context(),r,w,{{ pkg "errors" }}Wrap(err,"couldn't parse request"))
            return
        }

        ret,err := d.svc.{{ $m.GetName }}(r.Context(),&req)
        if err != nil {
            {{ pkg "httpruntime" }}SetError(r.Context(),r,w,{{ pkg "errors" }}Wrap(err,"returned from handler"))
            return
        }

        _,outbound := {{ pkg "httpruntime" }}MarshalerForRequest(r)
        w.Header().Set("Content-Type", outbound.ContentType())
        err = outbound.Marshal(w, ret)
        if err != nil {
            {{ pkg "httpruntime" }}SetError(r.Context(),r,w,{{ pkg "errors" }}Wrap(err,"couldn't write response"))
            return
        }
    })
    if isChi {
        chiMux.Method("{{ $b.HTTPMethod }}",pattern_goclay_{{ $svc.GetName }}_{{ $m.GetName }}_{{ $b.Index }}, h)
    } else {
        {{if $b.PathParams -}}
            panic("query URI params supported only for {{ pkg "chi" }}Router")
        {{- else -}}
            mux.Handle(pattern_goclay_{{ $svc.GetName }}_{{ $m.GetName }}_{{ $b.Index }}, {{ pkg "http" }}HandlerFunc(func(w {{ pkg "http" }}ResponseWriter, r *{{ pkg "http" }}Request) {
                if r.Method != "{{ $b.HTTPMethod }}" {
                    w.WriteHeader({{ pkg "http" }}StatusMethodNotAllowed)
                    return
                }
                h(w, r)
            }))
        {{- end }}
    }
    {{ end }}
    {{ end }}
}
{{ end }}
{{ end }} // base service handler ended
`))

	footerTemplate = template.Must(template.New("footer").Funcs(funcMap).Parse(`
    var _swaggerDef_{{ varName .GetName }} = []byte(` + "`" + `{{ escapeBackTicks (byteStr .SwaggerBuffer) }}` + `
` + "`)" + `
`))

	patternsTemplate = template.Must(template.New("patterns").Funcs(funcMap).Parse(`
{{define "base"}}
var (
{{range $svc := .Services}}
{{range $m := $svc.Methods}}
{{range $b := $m.Bindings}}

    pattern_goclay_{{$svc.GetName}}_{{$m.GetName}}_{{$b.Index}} = "{{$b.PathTmpl.Template}}"

    pattern_goclay_{{$svc.GetName}}_{{$m.GetName}}_{{$b.Index}}_builder = func(
        {{range $p := $b.PathParams -}}
            {{$p.Target.GetName}} {{toGoType $p.Target.GetType}},
        {{end -}}
    ) string {
        return {{ pkg "fmt" }}Sprintf("{{arrayToPathInterp $b.PathTmpl.Template}}",{{range $p := $b.PathParams}}{{$p.Target.GetName}},{{end}})
    }

    {{if not (hasAsterisk $b.ExplicitParams)}}
        unmarshaler_goclay_{{$svc.GetName}}_{{$m.GetName}}_{{$b.Index}}_boundParams = map[string]struct{}{
            {{ range $n := $b.ExplicitParams -}}
                "{{$n}}": struct{}{},
            {{ end }}
        }
    {{end}}

    unmarshaler_goclay_{{$svc.GetName}}_{{$m.GetName}}_{{$b.Index}} = func(r *{{ pkg "http" }}Request,req *{{$m.RequestType.GetName}}) error {
        {{if not (hasAsterisk $b.ExplicitParams)}}
            for k,v := range r.URL.Query() {
                if _,ok := unmarshaler_goclay_{{$svc.GetName}}_{{$m.GetName}}_{{$b.Index}}_boundParams[{{ pkg "strings" }}ToLower(k)];ok {
                    continue
                }
                if err := {{ pkg "errors" }}Wrap({{ pkg "runtime" }}PopulateFieldFromPath(req, k, v[0]), "couldn't populate field from Path"); err != nil {
                    return err
                }        
            }
        {{end}}
        {{- if $b.Body -}}
            {{- template "unmbody" . -}}
        {{- end -}}
        {{- if $b.PathParams -}}
            {{- template "unmpath" . -}}
        {{ end }}
        return nil
    }
{{ end }}
{{ end }}
{{ end }}
)
{{ end }}
{{define "unmbody"}}
    inbound,_ := {{ pkg "httpruntime" }}MarshalerForRequest(r)
    if err := {{ pkg "errors" }}Wrap(inbound.Unmarshal(r.Body,req),"couldn't read request JSON"); err != nil {
        return err
    }
{{end}}
{{define "unmpath"}}
    rctx := {{ pkg "chi" }}RouteContext(r.Context())
    if rctx == nil {
        panic("Only chi router is supported for GETs atm")
    }
    for pos,k := range rctx.URLParams.Keys {
        if err := {{ pkg "errors" }}Wrap({{ pkg "runtime" }}PopulateFieldFromPath(req, k, rctx.URLParams.Values[pos]), "couldn't populate field from Path"); err != nil {
            return err
        }
    }
{{end}}
`))

	implTemplate = template.Must(template.New("impl").Funcs(funcMap).Parse(`
// Code generated by protoc-gen-goclay, but your can (must) modify it.
// source: {{ .GetName }}

package  {{ .GoPkg.Name }}

import (
    {{ range $i := .Imports }}{{ if $i.Standard }}{{ $i | printf "%s\n" }}{{ end }}{{ end }}

    {{ range $i := .Imports }}{{ if not $i.Standard }}{{ $i | printf "%s\n" }}{{ end }}{{ end }}
)

{{ range $service := .Services }}

type {{ $service.GetName }}Implementation struct {}

func New{{ $service.GetName }}() *{{ $service.GetName }}Implementation {
    return &{{ $service.GetName }}Implementation{}
}

{{ range $method := $service.Methods }}
func (i *{{ $service.GetName }}Implementation) {{ $method.Name }}(ctx {{ pkg "context" }}Context, req *{{ pkg "desc" }}{{ $method.RequestType.GetName }}) (*{{ pkg "desc" }}{{ $method.ResponseType.GetName }}, error) {
    return nil, {{ pkg "errors" }}New("not implemented")
}
{{ end }}

// GetDescription is a simple alias to the ServiceDesc constructor.
// It makes it possible to register the service implementation @ the server.
func (i *{{ $service.GetName }}Implementation) GetDescription() {{ pkg "transport" }}ServiceDesc {
    return {{ pkg "desc" }}New{{ $service.GetName }}ServiceDesc(i)
}

{{ end }}
`))
	clientTemplate = template.Must(template.New("http-client").Funcs(funcMap).Parse(`
{{range $svc := .Services}}
type {{$svc.GetName}}_httpClient struct {
    c *{{ pkg "http" }}Client
    host string
}

// New{{$svc.GetName}}HTTPClient creates new HTTP client for {{$svc.GetName}}Server.
// Pass addr in format "http://host[:port]".
func New{{$svc.GetName}}HTTPClient(c *{{ pkg "http" }}Client,addr string) {{$svc.GetName}}Client {
    if {{ pkg "strings" }}HasSuffix(addr, "/") {
        addr = addr[:len(addr)-1]
    }
    return &{{$svc.GetName}}_httpClient{c:c,host:addr}
}
{{range $m := $svc.Methods}}
{{range $b := $m.Bindings}}
func (c *{{$svc.GetName}}_httpClient) {{$m.GetName}}(ctx {{ pkg "context" }}Context,in *{{$m.RequestType.GetName}},_ ...{{ pkg "grpc" }}CallOption) (*{{$m.ResponseType.GetName}},error) {
    path := pattern_goclay_{{$svc.GetName}}_{{$m.GetName}}_{{$b.Index}}_builder({{range $p := $b.PathParams}}in.{{goTypeName $p.String}},{{end}})

    buf := {{ pkg "bytes" }}NewBuffer(nil)

    m := {{ pkg "httpruntime" }}DefaultMarshaler(nil)
    err := m.Marshal(buf, in)
    if err != nil {
        return nil, {{ pkg "errors" }}Wrap(err, "can't marshal request")
    }

    req, err := {{ pkg "http" }}NewRequest("{{$b.HTTPMethod}}", c.host+path, buf)
    if err != nil {
        return nil, {{ pkg "errors" }}Wrap(err, "can't initiate HTTP request")
    }

    req.Header.Add("Accept", m.ContentType())

    rsp, err := c.c.Do(req)
    if err != nil {
        return nil, {{ pkg "errors" }}Wrap(err, "error from client")
    }
    defer rsp.Body.Close()

    if rsp.StatusCode>= 400 {
        b,_ := {{ pkg "ioutil" }}ReadAll(rsp.Body)
        return nil,{{ pkg "errors" }}Errorf("%v %v: server returned HTTP %v: '%v'",req.Method,req.URL.String(),rsp.StatusCode,string(b))
    }

    ret := &{{$m.ResponseType.GetName}}{}
    err = m.Unmarshal(rsp.Body, ret)
    return ret, {{ pkg "errors" }}Wrap(err, "can't unmarshal response")
}
{{end}}
{{end}}
{{end}}
`))
)
