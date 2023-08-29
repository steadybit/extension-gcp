{{/* vim: set filetype=mustache: */}}

{{/*
Expand the name of the chart.
*/}}
{{- define "gcp.secret.name" -}}
{{- default "steadybit-extension-gcp" .Values.gcp.existingSecret -}}
{{- end -}}
