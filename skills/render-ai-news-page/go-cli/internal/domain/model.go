// Package domain 定义新闻页面生成与 lint 的稳定领域模型。
package domain

import "fmt"

// ReportType 决定新闻稿使用日报还是周报的独立格式规则。
type ReportType string

const (
	ReportDaily  ReportType = "daily"
	ReportWeekly ReportType = "weekly"
)

// ExitCode 是 CLI 对外稳定退出码。
type ExitCode int

const (
	ExitSuccess ExitCode = 0
	ExitUsage   ExitCode = 2
	ExitLint    ExitCode = 3
	ExitIO      ExitCode = 4
)

// Issue 是一条可定位的 lint 诊断。
type Issue struct {
	Code    string
	Message string
}

// Error 表示带稳定退出码的顶层错误。
type Error struct {
	Code    ExitCode
	Message string
	Cause   error
}

func (e *Error) Error() string {
	if e.Cause == nil {
		return e.Message
	}
	return fmt.Sprintf("%s：%v", e.Message, e.Cause)
}

func (e *Error) Unwrap() error {
	return e.Cause
}
