{{- /*
Based on templates/markdown/type.tpl of github.com/elastic/crd-ref-docs v0.2.0
(Apache License 2.0), with two changes for NFD:
- the underlying type of an alias is rendered with markdownRenderType instead of
  markdownRenderTypeLink, so that slice and map aliases read
  "[FeatureMatcherTerm](#featurematcherterm) array" (not just the element type)
  and "object (keys:string, values:[MatchExpression](#matchexpression))" (not a
  link to an anchor that does not exist);
- enum constants with an empty value (MatchAny, TypeEmpty) are not listed: they
  stand for an unset field, and the API rejects an empty op.
*/ -}}
{{- define "type" -}}
{{- $type := . -}}
{{- if markdownShouldRenderType $type -}}

#### {{ $type.Name }}

{{ if $type.IsAlias }}_Underlying type:_ _{{ markdownRenderType $type.UnderlyingType }}_{{ end }}

{{ $type.Doc }}

{{ if $type.Validation -}}
_Validation:_
{{- range $type.Validation }}
- {{ . }}
{{- end }}
{{- end }}

{{ if $type.References -}}
_Appears in:_
{{- range $type.SortedReferences }}
- {{ markdownRenderTypeLink . }}
{{- end }}
{{- end }}

{{ if $type.Members -}}
| Field | Description | Default | Validation |
| --- | --- | --- | --- |
{{ if $type.GVK -}}
| `apiVersion` _string_ | `{{ $type.GVK.Group }}/{{ $type.GVK.Version }}` | | |
| `kind` _string_ | `{{ $type.GVK.Kind }}` | | |
{{ end -}}

{{ range $type.Members -}}
| `{{ .Name  }}` _{{ markdownRenderType .Type }}_ | {{ template "type_members" . }} | {{ markdownRenderDefault .Default }} | {{ range .Validation -}} {{ markdownRenderFieldDoc . }} <br />{{ end }} |
{{ end -}}

{{ end -}}

{{ if $type.EnumValues -}} 
| Field | Description |
| --- | --- |
{{ range $type.EnumValues -}}
{{ if .Name -}}
| `{{ .Name }}` | {{ markdownRenderFieldDoc .Doc }} |
{{ end -}}
{{ end -}}
{{ end -}}


{{- end -}}
{{- end -}}
