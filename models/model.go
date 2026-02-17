package models

import (
	"github.com/JoshPattman/jpf"
)

func TransformByPrefix(prefix string) func(string) string {
	return func(s string) string {
		return prefix + s
	}
}

func failedResponse() jpf.ModelResponse {
	failedUsage := jpf.Usage{FailedCalls: 1}
	return jpf.ModelResponse{Usage: failedUsage}
}

func failedResponseAfter(usage jpf.Usage) jpf.ModelResponse {
	failedUsage := jpf.Usage{FailedCalls: 1}.Add(usage)
	return jpf.ModelResponse{Usage: failedUsage}
}
