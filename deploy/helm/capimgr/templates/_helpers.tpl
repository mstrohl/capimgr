{{- define "capimgr.backendImage" -}}
{{ .Values.image.registry }}/{{ .Values.backend.repository }}:{{ .Values.image.tag }}
{{- end }}
{{- define "capimgr.frontendImage" -}}
{{ .Values.image.registry }}/{{ .Values.frontend.repository }}:{{ .Values.image.tag }}
{{- end }}
{{- define "capimgr.jobImage" -}}
{{ default (printf "%s/%s:%s" .Values.image.registry .Values.clusterctl.repository .Values.image.tag) .Values.clusterctlImage }}
{{- end }}
{{- define "capimgr.githubSecret" -}}
{{ default "capimgr-github" .Values.githubTokenSecret }}
{{- end }}
