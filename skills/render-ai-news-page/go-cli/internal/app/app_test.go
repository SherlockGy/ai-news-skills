package app

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"finance-skills/common/render-ai-news-page/go-cli/internal/domain"
)

const appValidSource = `# 企业 AI 工作变革速览

**生成时间：2026 年 8 月 13 日 06:00（北京时间）**

这是完整导语。

## 今日思想亮点

**为什么值得关注：**中文紧邻正文。

# 今日企业 AI 重要动态

## 1. 长标题用于验证自然换行并保留完整限定信息

正文及 [来源](https://example.com/news)。
`

const appWeeklySource = `# AI 科技周报

> **日期范围：2026 年 8 月 15 日至 2026 年 8 月 21 日**

## 本周判断

这是周报判断。

## 重点新闻

### 1. 周报新闻标题

正文及 [来源](https://example.com/weekly)。
`

func TestParseArgsContract(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		args      []string
		wantError string
		check     func(t *testing.T, got command)
	}{
		{name: "build", args: []string{"build", "--report-type", "daily", "--source", "news.md", "--output", "page.html"}, check: func(t *testing.T, got command) {
			if got.name != "build" || got.reportType != domain.ReportDaily || got.sourcePath != "news.md" || got.outputPath != "page.html" {
				t.Fatalf("parseArgs() = %#v", got)
			}
		}},
		{name: "lint", args: []string{"lint", "--report-type", "weekly", "--page", "page.html", "--source", "news.md"}, check: func(t *testing.T, got command) {
			if got.name != "lint" || got.reportType != domain.ReportWeekly || got.sourcePath != "news.md" || got.pagePath != "page.html" {
				t.Fatalf("parseArgs() = %#v", got)
			}
		}},
		{name: "root help", args: []string{"--help"}, check: func(t *testing.T, got command) {
			if !got.rootHelp {
				t.Fatal("未进入根帮助控制分支")
			}
		}},
		{name: "command help", args: []string{"build", "-h"}, check: func(t *testing.T, got command) {
			if !got.help || got.name != "build" {
				t.Fatalf("parseArgs() = %#v", got)
			}
		}},
		{name: "version", args: []string{"version"}, check: func(t *testing.T, got command) {
			if !got.version {
				t.Fatal("未进入版本控制分支")
			}
		}},
		{name: "empty", args: nil, wantError: "缺少命令"},
		{name: "unknown command", args: []string{"create"}, wantError: "未知命令"},
		{name: "version conflict", args: []string{"version", "--help"}, wantError: "不能重复或与其它参数混用"},
		{name: "help conflict", args: []string{"build", "--help", "--source", "news.md"}, wantError: "不能与业务参数混用"},
		{name: "help with unknown option", args: []string{"build", "--help", "--force"}, wantError: "未知参数"},
		{name: "help with extra position", args: []string{"build", "--help", "extra"}, wantError: "多余位置参数"},
		{name: "duplicate help", args: []string{"build", "--help", "--help"}, wantError: "不能重复"},
		{name: "root help with unknown option", args: []string{"--help", "--force"}, wantError: "未知参数"},
		{name: "unknown option", args: []string{"build", "--source", "news.md", "--output", "page.html", "--force"}, wantError: "未知参数"},
		{name: "wrong mode", args: []string{"lint", "--source", "news.md", "--output", "page.html"}, wantError: "未知参数"},
		{name: "extra position", args: []string{"build", "--source", "news.md", "--output", "page.html", "extra"}, wantError: "多余位置参数"},
		{name: "duplicate source", args: []string{"build", "--source", "one.md", "--source", "two.md", "--output", "page.html"}, wantError: "不能重复"},
		{name: "duplicate output", args: []string{"build", "--source", "news.md", "--output", "one.html", "--output", "two.html"}, wantError: "不能重复"},
		{name: "duplicate page", args: []string{"lint", "--source", "news.md", "--page", "one.html", "--page", "two.html"}, wantError: "不能重复"},
		{name: "duplicate report type", args: []string{"build", "--report-type", "daily", "--report-type", "weekly", "--source", "news.md", "--output", "page.html"}, wantError: "不能重复"},
		{name: "invalid report type", args: []string{"build", "--report-type", "monthly", "--source", "news.md", "--output", "page.html"}, wantError: "只允许 daily 或 weekly"},
		{name: "missing report type", args: []string{"build", "--source", "news.md", "--output", "page.html"}, wantError: "--report-type"},
		{name: "missing source", args: []string{"build", "--report-type", "daily", "--output", "page.html"}, wantError: "--source"},
		{name: "missing output", args: []string{"build", "--report-type", "daily", "--source", "news.md"}, wantError: "--output"},
		{name: "missing page", args: []string{"lint", "--report-type", "daily", "--source", "news.md"}, wantError: "--page"},
		{name: "empty value", args: []string{"build", "--source", "", "--output", "page.html"}, wantError: "不能为空"},
		{name: "equals syntax rejected", args: []string{"build", "--source=news.md", "--output", "page.html"}, wantError: "未知参数"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			got, err := parseArgs(test.args)
			if test.wantError != "" {
				if err == nil || !strings.Contains(err.Error(), test.wantError) {
					t.Fatalf("parseArgs() error = %v，期望包含 %q", err, test.wantError)
				}
				return
			}
			if err != nil {
				t.Fatalf("parseArgs() error = %v", err)
			}
			test.check(t, got)
		})
	}
}

func TestBuildAvoidsExistingOutputAndLintReadsActualPage(t *testing.T) {
	t.Parallel()

	temp := t.TempDir()
	sourcePath := filepath.Join(temp, "新闻.md")
	outputPath := filepath.Join(temp, "日报.html")
	if err := os.WriteFile(sourcePath, []byte(appValidSource), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(outputPath, []byte("已有文件"), 0o644); err != nil {
		t.Fatal(err)
	}

	var stdout, stderr bytes.Buffer
	exitCode := Run([]string{"build", "--report-type", "daily", "--source", sourcePath, "--output", outputPath}, &stdout, &stderr)
	if exitCode != int(domain.ExitSuccess) {
		t.Fatalf("Run(build) exit = %d, stderr = %s", exitCode, stderr.String())
	}
	actualPath := filepath.Join(temp, "日报-2.html")
	if !strings.Contains(stdout.String(), "输出文件: "+actualPath) {
		t.Fatalf("build stdout = %s", stdout.String())
	}
	original, err := os.ReadFile(outputPath)
	if err != nil || string(original) != "已有文件" {
		t.Fatalf("已有文件被覆盖：%q, error = %v", original, err)
	}

	stdout.Reset()
	stderr.Reset()
	exitCode = Run([]string{"lint", "--report-type", "daily", "--source", sourcePath, "--page", actualPath}, &stdout, &stderr)
	if exitCode != int(domain.ExitSuccess) || !strings.Contains(stdout.String(), "lint: 通过") {
		t.Fatalf("Run(lint) exit = %d, stdout = %s, stderr = %s", exitCode, stdout.String(), stderr.String())
	}
}

func TestBuildLintFailureCreatesNoOutput(t *testing.T) {
	t.Parallel()

	temp := t.TempDir()
	sourcePath := filepath.Join(temp, "invalid.md")
	outputPath := filepath.Join(temp, "invalid.html")
	if err := os.WriteFile(sourcePath, []byte("# 只有标题\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	var stdout, stderr bytes.Buffer
	exitCode := Run([]string{"build", "--report-type", "daily", "--source", sourcePath, "--output", outputPath}, &stdout, &stderr)
	if exitCode != int(domain.ExitLint) {
		t.Fatalf("Run(build) exit = %d, stderr = %s", exitCode, stderr.String())
	}
	if _, err := os.Stat(outputPath); !os.IsNotExist(err) {
		t.Fatalf("lint 失败后仍产生输出：%v", err)
	}
}

func TestWeeklyBuildAndLintRequireWeeklyRules(t *testing.T) {
	t.Parallel()

	temp := t.TempDir()
	sourcePath := filepath.Join(temp, "周报.md")
	outputPath := filepath.Join(temp, "周报.html")
	if err := os.WriteFile(sourcePath, []byte(appWeeklySource), 0o644); err != nil {
		t.Fatal(err)
	}

	var stdout, stderr bytes.Buffer
	wrongModeOutput := filepath.Join(temp, "错误模式.html")
	exitCode := Run([]string{"build", "--report-type", "daily", "--source", sourcePath, "--output", wrongModeOutput}, &stdout, &stderr)
	if exitCode != int(domain.ExitLint) {
		t.Fatalf("周报使用日报规则 build exit = %d, stderr = %s", exitCode, stderr.String())
	}
	if _, err := os.Stat(wrongModeOutput); !os.IsNotExist(err) {
		t.Fatalf("模式不匹配后仍产生输出：%v", err)
	}

	stdout.Reset()
	stderr.Reset()
	exitCode = Run([]string{"build", "--report-type", "weekly", "--source", sourcePath, "--output", outputPath}, &stdout, &stderr)
	if exitCode != int(domain.ExitSuccess) || !strings.Contains(stdout.String(), "报表类型: weekly") {
		t.Fatalf("Run(weekly build) exit = %d, stdout = %s, stderr = %s", exitCode, stdout.String(), stderr.String())
	}

	stdout.Reset()
	stderr.Reset()
	exitCode = Run([]string{"lint", "--report-type", "weekly", "--source", sourcePath, "--page", outputPath}, &stdout, &stderr)
	if exitCode != int(domain.ExitSuccess) || !strings.Contains(stdout.String(), "lint: 通过") {
		t.Fatalf("Run(weekly lint) exit = %d, stdout = %s, stderr = %s", exitCode, stdout.String(), stderr.String())
	}

	stdout.Reset()
	stderr.Reset()
	exitCode = Run([]string{"lint", "--report-type", "daily", "--source", sourcePath, "--page", outputPath}, &stdout, &stderr)
	if exitCode != int(domain.ExitLint) || !strings.Contains(stderr.String(), "SOURCE_DATE") {
		t.Fatalf("周报被日报规则接受：exit = %d, stdout = %s, stderr = %s", exitCode, stdout.String(), stderr.String())
	}
}

func TestWriteVerificationFailureRemovesCreatedFile(t *testing.T) {
	t.Parallel()

	temp := t.TempDir()
	outputPath := filepath.Join(temp, "待校验.html")
	_, err := writeVerifiedAvoidingCollision(outputPath, []byte("完整页面"), func(actual []byte) error {
		if string(actual) != "完整页面" {
			t.Fatalf("回读内容 = %q", actual)
		}
		return fmt.Errorf("模拟最终 lint 失败")
	})
	if err == nil || !strings.Contains(err.Error(), "模拟最终 lint 失败") {
		t.Fatalf("writeVerifiedAvoidingCollision() error = %v", err)
	}
	if _, statErr := os.Stat(outputPath); !os.IsNotExist(statErr) {
		t.Fatalf("最终 lint 失败后仍残留输出：%v", statErr)
	}
}

func TestControlModesMakeNoBusinessCalls(t *testing.T) {
	t.Parallel()

	for _, args := range [][]string{{"--help"}, {"build", "--help"}, {"lint", "-h"}, {"version"}} {
		var stdout, stderr bytes.Buffer
		if exitCode := Run(args, &stdout, &stderr); exitCode != int(domain.ExitSuccess) {
			t.Fatalf("Run(%v) exit = %d, stderr = %s", args, exitCode, stderr.String())
		}
	}
	var stdout, stderr bytes.Buffer
	if exitCode := Run([]string{"build", "--help", "--source", "不存在.md"}, &stdout, &stderr); exitCode != int(domain.ExitUsage) {
		t.Fatalf("帮助冲突 exit = %d", exitCode)
	}
}

func TestIssueOutputIsBounded(t *testing.T) {
	t.Parallel()

	issues := make([]domain.Issue, 105)
	for index := range issues {
		issues[index] = domain.Issue{Code: "TEST", Message: fmt.Sprintf("错误-%d", index+1)}
	}
	var output bytes.Buffer
	writeIssues(&output, issues)
	if strings.Contains(output.String(), "错误-101") || !strings.Contains(output.String(), "已省略 5 个错误") {
		t.Fatalf("writeIssues() = %s", output.String())
	}
}
