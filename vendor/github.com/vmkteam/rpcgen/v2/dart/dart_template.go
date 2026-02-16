package dart

const client = `/// Code generated from jsonrpc schema by rpcgen v{{ .Version }} with {{ .Lang }} v{{ .LocalVersion }}; DO NOT EDIT.

import 'package:json_annotation/json_annotation.dart';
import 'package:smd_annotations/annotations.dart';

part '{{ .Part }}.g.dart';
{{ range .Models }}
{{- range .Description }}
/// {{ . }}
{{- end }}
@JsonSerializable()
class {{ .Type }} extends Equatable {
  {{- range .Properties }}
  {{- range .Description }}
  /// {{ . }}
  {{- end }}
  @JsonKey(name: '{{ .Name }}')
  final {{ .Type }}{{ if .Optional }}?{{ end }} {{ .Name }};
  {{- end }}

  {{ .Type }}({
  {{- range .Properties }}
    {{ if ne .Optional true }}required {{ end }}this.{{ .Name }},
  {{- end }}
  });

  Map<String, dynamic> toJson() => _${{ .Type }}ToJson(this);

  factory {{ .Type }}.fromJson(Map<String, dynamic> json) =>
      _${{ .Type }}FromJson(json);

  @override
  List<Object?> get props => [
        {{- range .Properties }}
        {{ .Name }},
        {{- end }}
      ];
}
{{ end }}

{{- range .Namespaces }}
{{- $namespaceName := .Name }}
{{- range .Methods }}
{{- $paramsLen := len .Parameters }}
{{- if ne $paramsLen 0 }}
@JsonSerializable()
class {{ title $namespaceName }}{{ .ParamsClass }} {
  {{- range .Parameters }}
  {{- range .Description }}
  /// {{ . }}
  {{- end }}
  @JsonKey(name: '{{ .Name }}')
  final {{ .Type }}{{ if .Optional }}?{{ end }} {{ .Name }};
  {{- end }}

  {{ title $namespaceName }}{{ .ParamsClass }}({
  {{- range .Parameters }}
    {{ if ne .Optional true }}required {{ end }}this.{{ .Name }},
  {{- end }}
  });

  Map<String, dynamic> toJson() => _${{ title $namespaceName }}{{ .ParamsClass }}ToJson(this);

  factory {{ title $namespaceName }}{{ .ParamsClass }}.fromJson(Map<String, dynamic> json) =>
      _${{ title $namespaceName }}{{ .ParamsClass }}FromJson(json);
}
{{ end }}
{{- end }}
{{- end }}

{{ range .Namespaces }}
{{- $namespaceName := .Name }}
{{ title .Name }}RPC {{ .Name }}RPCInstance({required RPC rpc}) => _{{ title .Name }}RPC(rpc: rpc);
@RPCNamespace('{{ .Name }}')
abstract class {{ title .Name }}RPC {
{{- range .Methods }}{{- $paramsLen := len .Parameters }}
  {{- range .Description }}
  {{- if ne . "" }}
  /// {{ . }}
  {{- end }}
  {{- end }}
  @RPCMethod('{{ title .SafeName }}')
  Future<{{ .Returns.Type }}{{ if .Returns.Optional }}?{{ end }}> {{ camelCase .SafeName }}({{ if ne $paramsLen 0 }}{{ title $namespaceName }}{{ .ParamsClass }} params{{ end }});

{{- end }}
}
{{ end }}`
