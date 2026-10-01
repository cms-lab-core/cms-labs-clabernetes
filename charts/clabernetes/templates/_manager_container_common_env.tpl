{{/*
launcherImage is the image direct device Pods run their per-node helper containers from. It lives
in one place because two consumers need it: the manager's environment, which is where the runtime
falls back to when no Config declares a launcher, and the Config's launcher block, which is what
makes the configured image visible in the cluster instead of only in the manager's environment.
*/}}
{{- define "launcherImage" -}}
{{- $configured := "" -}}
{{- with .Values.globalConfig -}}
{{- with .deployment -}}
{{- with .launcher -}}
{{- $configured = .image -}}
{{- end -}}
{{- end -}}
{{- end -}}
{{- if $configured -}}
{{- $configured -}}
{{- else if eq .Chart.Version "0.0.0" -}}
"ghcr.io/cms-lab-core/cms-labs-clabernetes/clabernetes-launcher:dev-latest"
{{- else -}}
"ghcr.io/cms-lab-core/cms-labs-clabernetes/clabernetes-launcher:{{ .Chart.Version }}"
{{- end -}}
{{- end -}}

{{- define "managerContainerCommonEnv" -}}
- name: APP_NAME
  value: {{ .Values.appName }}
- name: POD_NAME
  valueFrom:
    fieldRef:
      fieldPath: metadata.name
- name: POD_NAMESPACE
  valueFrom:
    fieldRef:
      fieldPath: metadata.namespace
- name: CLIENT_OPERATION_TIMEOUT_MULTIPLIER
  value: "{{ .Values.manager.clientOperationTimeoutMultiplier }}"
- name: MANAGER_LOGGER_LEVEL
  value: {{ .Values.manager.managerLogLevel }}
- name: CONTROLLER_LOGGER_LEVEL
  value: {{ .Values.manager.controllerLogLevel }}
- name: NODE_RUNTIME_IMAGE
  {{- if .Values.manager.image }}
  value: {{ .Values.manager.image }}
  {{- else if eq .Chart.Version "0.0.0" }}
  value: "ghcr.io/cms-lab-core/cms-labs-clabernetes/clabernetes-manager:dev-latest"
  {{- else }}
  value: "ghcr.io/cms-lab-core/cms-labs-clabernetes/clabernetes-manager:{{ .Chart.Version }}"
  {{- end }}
- name: LAUNCHER_IMAGE
  value: {{ include "launcherImage" . }}
- name: PLANNER_POOL_ENABLED
  value: {{ .Values.plannerPool.enabled | quote }}
- name: C9S_DIAGNOSTICS
  value: {{ .Values.manager.diagnostics | default false | quote }}
{{- end -}}
