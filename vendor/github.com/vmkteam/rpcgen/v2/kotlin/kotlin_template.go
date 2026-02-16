package kotlin

const model = `/// Code generated from jsonrpc schema by rpcgen v{{ .Version }} with {{ .Lang }} v{{ .LocalVersion }}; DO NOT EDIT.
package {{ .PackageAPI }}.model

import java.time.LocalTime
import java.time.ZonedDateTime

{{ range $model := .Models }}
{{- if $model.Description }}
/// {{ $model.Description }}
{{- end }}
data class {{ $model.Name }}(
    {{- $fieldsLen := len $model.Fields }}{{- range $model.Fields }}
    {{- if .Description }}
    /**
     * {{ .Description }}
     */
    {{- end }}
    {{- if $model.IsInitial }}
    val {{ .Name }}: {{ .Type }}{{ if .Optional }}? = null{{ else }} = {{ .DefaultValue}}{{ if .IsObject}}(){{ else }}{{ end }}{{ end }},
    {{- else }}
    val {{ .Name }}: {{ .Type }}{{ if .Optional }}?{{ else }}{{ end }},
    {{- end }}
    {{- end }}
)
{{ end }}
`
const protocolTemplate = `/// Code generated from jsonrpc schema by rpcgen v{{ .Version }} with {{ .Lang }} v{{ .LocalVersion }}; DO NOT EDIT.
package {{ .PackageAPI }}

import com.google.gson.reflect.TypeToken
import java.time.ZonedDateTime
import java.time.LocalTime
import {{ .PackageAPI }}.model.*
{{- range .Imports }}
import {{.}}
{{- end }}

interface {{ .Class }} : Transport {
{{- range .Methods }}
{{  if  hasDescriptions . }}
    /**
{{- range .Description }}
    {{- if ne . "" }}
     * {{ . }}
    {{- end }}
{{- end }}
{{- if  (len .Errors) }}
     *
     * Коды ошибок:
     * 
    {{- range .Errors }}
     *    "{{.Name}}": "{{.Description}}", 
    {{- end }}
     *
     *
{{- end -}}
{{ if (len .Parameters) }}
    {{- range .Parameters }}
        {{- if ne .Description "" }}
     * @param {{ .Name }} 
        {{- end }}
    {{- end }}
{{- end }}
     * @return 
     */
{{- end }}
    fun {{ .SafeName }}(
{{- range .Parameters}}
        {{ .Name }}: {{ .Type }}{{ if .Optional }}?{{ end }},
{{- end}}
        vararg transportOptions: TransportOption,
    ) = request(
        transportOptions,
        object : TypeToken<ApiResponse<{{- if ne .Returns.Type "" }}{{ .Returns.Type }}{{else}}Nothing{{end}}>>() {},
        "{{ .Name }}",
{{- range .Parameters}}
        "{{ .Name }}" to {{ .Name }},
{{- end}}
    )
{{- end }}
}
`
