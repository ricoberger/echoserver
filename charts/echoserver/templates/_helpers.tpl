{{/*
Expand the name of the chart.
*/}}
{{- define "echoserver.name" -}}
{{- default .Chart.Name .Values.nameOverride | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Create a default fully qualified app name.
We truncate at 63 chars because some Kubernetes name fields are limited to this (by the DNS naming spec).
If release name contains chart name it will be used as a full name.
*/}}
{{- define "echoserver.fullname" -}}
{{- if .Values.fullnameOverride }}
{{- .Values.fullnameOverride | trunc 63 | trimSuffix "-" }}
{{- else }}
{{- $name := default .Chart.Name .Values.nameOverride }}
{{- if contains $name .Release.Name }}
{{- .Release.Name | trunc 63 | trimSuffix "-" }}
{{- else }}
{{- printf "%s-%s" .Release.Name $name | trunc 63 | trimSuffix "-" }}
{{- end }}
{{- end }}
{{- end }}

{{/*
Create chart name and version as used by the chart label.
*/}}
{{- define "echoserver.chart" -}}
{{- printf "%s-%s" .Chart.Name .Chart.Version | replace "+" "_" | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Common labels
*/}}
{{- define "echoserver.labels" -}}
helm.sh/chart: {{ include "echoserver.chart" . }}
{{ include "echoserver.selectorLabels" . }}
{{- if .Chart.AppVersion }}
app.kubernetes.io/version: {{ .Chart.AppVersion | quote }}
{{- end }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
{{- end }}

{{/*
Selector labels
*/}}
{{- define "echoserver.selectorLabels" -}}
app.kubernetes.io/name: {{ include "echoserver.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
{{- end }}


{{/*
Additional annotations for Pods
*/}}
{{- define "echoserver.podAnnotations" -}}
{{- if .Values.podAnnotations }}
{{- toYaml .Values.podAnnotations }}
{{- end }}
{{- end }}

{{/*
Additional labels for Pods
*/}}
{{- define "echoserver.podLabels" -}}
{{- if .Values.podLabels }}
{{- toYaml .Values.podLabels }}
{{- end }}
{{- end }}

{{/*
Additional annotations for the Service
*/}}
{{- define "echoserver.serviceAnnotations" -}}
{{- if .Values.service.annotations }}
{{- toYaml .Values.service.annotations }}
{{- end }}
{{- end }}

{{/*
Additional labels for the Service
*/}}
{{- define "echoserver.serviceLabels" -}}
{{- if .Values.service.labels }}
{{- toYaml .Values.service.labels }}
{{- end }}
{{- end }}

{{/*
Additional labels for the Service Monitor
*/}}
{{- define "echoserver.serviceMonitorLabels" -}}
{{- if .Values.serviceMonitor.labels }}
{{- toYaml .Values.serviceMonitor.labels }}
{{- end }}
{{- end }}
