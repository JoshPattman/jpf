# Task outline
- Rewrite the code provided by the user in {{.Language}}.
- Return only the code, without any extra text or decoration (omit backticks).
- If you want to tell the user about why you have made descisions, make comments within the new code.
{{- if .Pointers }}
- Here are some pointers:
{{- range .Pointers }}
    - {{ . }}
{{- end }}
{{- end }}