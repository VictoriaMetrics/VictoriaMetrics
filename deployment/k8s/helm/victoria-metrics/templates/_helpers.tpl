{{/* vim: set filetype=mustache: */}}
{{/*
Expand the name of the chart.
*/}}
{{- define "victoria-metrics.name" -}}
{{- default .Chart.Name .Values.nameOverride | trunc 63 | trimSuffix "-" -}}
{{- end -}}

{{/*
Create chart name and version as used by the chart label.
*/}}
{{- define "victoria-metrics.chart" -}}
{{- printf "%s-%s" .Chart.Name .Chart.Version | replace "+" "_" | trunc 63 | trimSuffix "-" -}}
{{- end -}}

{{/*
Create unified labels for victoria-metrics components
*/}}
{{- define "victoria-metrics.common.matchLabels" -}}
app.kubernetes.io/name: {{ include "victoria-metrics.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
{{- end -}}

{{- define "victoria-metrics.common.metaLabels" -}}
helm.sh/chart: {{ include "victoria-metrics.chart" . }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
{{- end -}}

{{- define "victoria-metrics.vmstorage.labels" -}}
{{ include "victoria-metrics.vmstorage.matchLabels" . }}
{{ include "victoria-metrics.common.metaLabels" . }}
{{- end -}}

{{- define "victoria-metrics.vmstorage.matchLabels" -}}
app: {{ .Values.vmstorage.name }}
{{ include "victoria-metrics.common.matchLabels" . }}
{{- end -}}

{{- define "victoria-metrics.vmselect.labels" -}}
{{ include "victoria-metrics.vmselect.matchLabels" . }}
{{ include "victoria-metrics.common.metaLabels" . }}
{{- end -}}

{{- define "victoria-metrics.vmselect.matchLabels" -}}
app: {{ .Values.vmselect.name }}
{{ include "victoria-metrics.common.matchLabels" . }}
{{- end -}}

{{- define "victoria-metrics.vminsert.labels" -}}
{{ include "victoria-metrics.vminsert.matchLabels" . }}
{{ include "victoria-metrics.common.metaLabels" . }}
{{- end -}}

{{- define "victoria-metrics.vminsert.matchLabels" -}}
app: {{ .Values.vminsert.name }}
{{ include "victoria-metrics.common.matchLabels" . }}
{{- end -}}

{{/*
Create a fully qualified vmstorage name.
We truncate at 63 chars because some Kubernetes name fields are limited to this (by the DNS naming spec).
*/}}
{{- define "victoria-metrics.vmstorage.fullname" -}}
{{- if .Values.vmstorage.fullnameOverride -}}
{{- .Values.vmstorage.fullnameOverride | trunc 63 | trimSuffix "-" -}}
{{- else -}}
{{- $name := default .Chart.Name .Values.nameOverride -}}
{{- if contains $name .Release.Name -}}
{{- printf "%s-%s" .Release.Name .Values.vmstorage.name | trunc 63 | trimSuffix "-" -}}
{{- else -}}
{{- printf "%s-%s-%s" .Release.Name $name .Values.vmstorage.name | trunc 63 | trimSuffix "-" -}}
{{- end -}}
{{- end -}}
{{- end -}}

{{/*
Create a fully qualified vmselect name.
We truncate at 63 chars because some Kubernetes name fields are limited to this (by the DNS naming spec).
*/}}
{{- define "victoria-metrics.vmselect.fullname" -}}
{{- if .Values.vmselect.fullnameOverride -}}
{{- .Values.vmselect.fullnameOverride | trunc 63 | trimSuffix "-" -}}
{{- else -}}
{{- $name := default .Chart.Name .Values.nameOverride -}}
{{- if contains $name .Release.Name -}}
{{- printf "%s-%s" .Release.Name .Values.vmselect.name | trunc 63 | trimSuffix "-" -}}
{{- else -}}
{{- printf "%s-%s-%s" .Release.Name $name .Values.vmselect.name | trunc 63 | trimSuffix "-" -}}
{{- end -}}
{{- end -}}
{{- end -}}

{{/*
Create a fully qualified vmselect name.
We truncate at 63 chars because some Kubernetes name fields are limited to this (by the DNS naming spec).
*/}}
{{- define "victoria-metrics.vminsert.fullname" -}}
{{- if .Values.vminsert.fullnameOverride -}}
{{- .Values.vminsert.fullnameOverride | trunc 63 | trimSuffix "-" -}}
{{- else -}}
{{- $name := default .Chart.Name .Values.nameOverride -}}
{{- if contains $name .Release.Name -}}
{{- printf "%s-%s" .Release.Name .Values.vminsert.name | trunc 63 | trimSuffix "-" -}}
{{- else -}}
{{- printf "%s-%s-%s" .Release.Name $name .Values.vminsert.name | trunc 63 | trimSuffix "-" -}}
{{- end -}}
{{- end -}}
{{- end -}}

{{- define "victoria-metrics.vminsert.vmstorage-pod-fqdn" -}}
{{- $pod := include "victoria-metrics.vmstorage.fullname" . -}}
{{- $svc := include "victoria-metrics.vmstorage.fullname" . -}}
{{- $namespace := .Release.Namespace -}}
{{- $dnsSuffix := .Values.clusterDomainSuffix -}}
{{- range $i := until (.Values.vmstorage.replicaCount | int) -}}
{{- printf "- --storageNode=%s-%d.%s.%s.svc.%s:8400\n" $pod $i $svc $namespace $dnsSuffix -}}
{{- end -}}
{{- end -}}

{{- define "victoria-metrics.vmselect.vmstorage-pod-fqdn" -}}
{{- $pod := include "victoria-metrics.vmstorage.fullname" . -}}
{{- $svc := include "victoria-metrics.vmstorage.fullname" . -}}
{{- $namespace := .Release.Namespace -}}
{{- $dnsSuffix := .Values.clusterDomainSuffix -}}
{{- range $i := until (.Values.vmstorage.replicaCount | int) -}}
{{- printf "- --storageNode=%s-%d.%s.%s.svc.%s:8401\n" $pod $i $svc $namespace $dnsSuffix -}}
{{- end -}}
{{- end -}}
