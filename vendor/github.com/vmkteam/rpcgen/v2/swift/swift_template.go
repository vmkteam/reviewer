package swift

const client = `/// Code generated from jsonrpc schema by rpcgen v{{ .Version }} with {{ .Lang }} v{{ .LocalVersion }}; DO NOT EDIT.

import Foundation

extension {{ .Class }}: RPCMethod {
    public var rpcMethod: String {
        switch self {
        case .batch(let requests): return requests.compactMap { $0.rpcMethod }.joined(separator: ",")
       	{{- range .Methods }}
        case .{{ .SafeName }}: return "{{ .Name }}"
       	{{- end }}
        }
    }
}

extension {{ .Class }}: RPCParameters {
    public var rpcParameters: [String: Any?]? {
        switch self {
        case .batch:
              return nil
{{- $methodsLen := len .Methods }}
{{- range $idx, $m := .Methods }}{{- $paramsLen := len .Parameters }}
        case {{ if gt $paramsLen 0 }}let {{ end }}.{{ .SafeName }}{{ if gt $paramsLen 0 }}({{ range .Parameters }}{{ .Name }}, {{ end }}_){{ end }}:
            return {{ if eq $paramsLen 0 }}nil{{ else }}[{{ range $index, $item := .Parameters }}"{{ .Name }}": {{ .Name }}{{ if or .IsArray .IsObject }}.any{{ end }}{{ if (notLast $index $paramsLen) }}, {{ end }}{{ end }}]{{ end }}
{{- if (notLast $idx $methodsLen) }}{{ print "\n" }}{{- end }}
{{- end }}
        }
    }
}

public enum {{ .Class }}: Codable, Hashable {
    /// Make batch requests.
    case batch(requests: [{{ .Class }}])
{{ range .Methods }}{{- $paramsLen := len .Parameters }}
    {{- range .Description }}
    {{- if ne . "" }}
    /// {{ . }}
    {{- end }}
    {{- end }}
    {{- if ne .Returns.Type ""}}
    /// - Returns: {{ .Returns.Type }}{{ if .Returns.Optional }}?{{ end }}
    {{- end }}
	case {{ .SafeName }}({{ if gt $paramsLen 0 }}{{ range .Parameters }}{{ .Name }}: {{ .Type }}{{ if .Optional }}?{{ end }}, {{ end }}{{ end }}requestId: String? = nil){{ end }}
}

{{ range .Models }}
{{- if .Description }}
/// {{ .Description }}
{{- end }}
public struct {{ .Name }}: Codable, Hashable {
    {{- $fieldsLen := len .Fields }}{{- range .Fields }}
    {{- if .Description }}
    /// {{ .Description }}
    {{- end }}
    {{- if and (not .Optional) (ne .DecodableDefault "") }}
    @DecodableDefault.{{ .DecodableDefault }}
    {{- end }}
    var {{ if .NeedEscaping }}{{ .SafeName }}{{ else }}{{ .Name }}{{ end }}: {{ .Type }}{{ if .Optional }}?{{ end }}
    {{- end }}
    init({{ range $index, $f := .Fields }}{{ .Name }}: {{ .Type }}{{ if .Optional }}? = nil{{ end }}{{ if (notLast $index $fieldsLen) }}, {{ end }}{{ end }}) {
        {{- range .Fields }}
        self.{{ .Name }} = {{ if .NeedEscaping }}{{ .SafeName }}{{ else }}{{ .Name }}{{ end }}
        {{- end }}
    }
}
{{ end }}

extension {{ .Class }} {
  public var rpcId: String? {
    switch self {
    case .batch:
      return nil

    case 
    {{- range $i, $method := .Methods -}}
        {{- if gt $i 0 }},
       {{ end -}}
        .{{ $method.SafeName }}({{ range $method.Parameters }}_, {{ end }}let requestId)
    {{- end }}:
          return requestId
    }
  }
}
`

const protocolTemplate = `/// Code generated from jsonrpc schema by rpcgen v{{ .Version }} with {{ .Lang }} v{{ .LocalVersion }}; DO NOT EDIT.

import Foundation
{{- range $service := .Namespaces }}

protocol {{ title $service.Namespace }}Networking {
{{- range $method := $service.Methods }}
    {{- range .Description }}
    {{- if ne . "" }}
    /// {{ . }}
    {{- end }}
    {{- end }}
    func {{ $method.SafeName }}({{- range $index, $item := $method.Parameters }}{{ $item.Name }}: {{ $item.Type }}{{ if $item.Optional }}?{{ end }}{{ if (notLast $index (len $method.Parameters)) }}, {{ end }}{{ end }}) async -> {{ if $method.Returns.Type }}Result<{{ $method.Returns.Type }}, RpcError>{{ else }}RpcError?{{ end }}
{{- end }}
}

extension Networking: {{ title $service.Namespace }}Networking {
{{- range $idx, $method := $service.Methods }}
    {{- range .Description }}
    {{- if ne . "" }}
    /// {{ . }}
    {{- end }}
    {{- end }}
	{{- if hasParamDescriptions .Parameters }}
	/// - Parameters:
	{{- range .Parameters }}
	{{- if ne .Description "" }}
	///  - {{ .Name }} : {{ .Description }}
	{{- end }}
	{{- end }}
	{{- end }}
	/// - Returns: Result<{{ if $method.Returns.Type }}{{ $method.Returns.Type }}, {{ else }}{{ end }}RpcError>
    func {{  $method.SafeName }}(
        {{- range $index, $item := $method.Parameters -}}
            {{ $item.Name }}: {{ $item.Type }}{{ if $item.Optional }}? = nil{{ end }}{{ if (notLast $index (len $method.Parameters)) }}, {{ end }}
        {{- end -}}
    ) async -> {{ if $method.Returns.Type }}Result<{{ $method.Returns.Type }}, RpcError>{{ else }}RpcError?{{ end }} {
        await request(.{{ $method.SafeName }}({{ if $method.Parameters }}{{- range $index, $item := $method.Parameters }}{{ $item.Name }}: {{ $item.Name }}{{ if (notLast $index (len $method.Parameters)) }}, {{ end }}{{ end }}{{ else }}{{ end }}))
    }
{{- if (notLast $idx (len $service.Methods)) }}
{{ end }}
{{- end }}
}
{{ end }}`
