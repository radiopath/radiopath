{{/* Image tags may carry semver build metadata; "+" is invalid in a label value. */}}
{{- define "radiopath.version" -}}
{{- .Values.image.tag | default .Chart.AppVersion | replace "+" "_" | trunc 63 | trimSuffix "-" -}}
{{- end }}

{{- define "radiopath.labels" -}}
app.kubernetes.io/name: {{ .Chart.Name }}
app.kubernetes.io/instance: {{ .Release.Name }}
app.kubernetes.io/version: {{ include "radiopath.version" . | quote }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
helm.sh/chart: {{ printf "%s-%s" .Chart.Name .Chart.Version }}
{{- end }}

{{/* Selector labels per component: web, worker, valkey. Never add the version here. */}}
{{- define "radiopath.selectorLabels" -}}
app.kubernetes.io/name: {{ .root.Chart.Name }}
app.kubernetes.io/instance: {{ .root.Release.Name }}
app.kubernetes.io/component: {{ .component }}
{{- end }}

{{- define "radiopath.image" -}}
{{ required "image.repository is required" .Values.image.repository }}:{{ required "image.tag is required" .Values.image.tag }}
{{- end }}

{{/* Env block shared by web and worker: values.env, per-component env, Valkey URL, workers. */}}
{{- define "radiopath.env" -}}
{{- $env := merge (dict) .component.env .root.Values.env }}
{{- range $name, $value := $env }}
- name: {{ $name | quote }}
  value: {{ $value | quote }}
{{- end }}
- name: RADIOPATH_WORKERS
  value: {{ .workers | quote }}
{{- if .root.Values.valkey.enabled }}
- name: RADIOPATH_REDIS_URL
  value: "redis://{{ .root.Release.Name }}-valkey:6379/0"
{{- end }}
{{- end }}

{{/* Pods the Service routes to: web pods, plus worker pods with worker.serveWeb. */}}
{{- define "radiopath.serviceSelector" -}}
app.kubernetes.io/name: {{ .Chart.Name }}
app.kubernetes.io/instance: {{ .Release.Name }}
radiopath/serves-web: "true"
{{- end }}
