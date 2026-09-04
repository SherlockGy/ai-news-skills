// Package app 实现 CLI 命令契约、输入校验和文件落盘边界。
package app

import (
	"bytes"
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"unicode/utf8"

	"finance-skills/common/render-ai-news-page/go-cli/assets"
	"finance-skills/common/render-ai-news-page/go-cli/internal/domain"
	"finance-skills/common/render-ai-news-page/go-cli/internal/page"
)

const (
	Version   = "1.1.1"
	logPrefix = "[renderAiNewsPage AI 新闻单页面]"
	issueCap  = 100
)

type command struct {
	name       string
	sourcePath string
	outputPath string
	pagePath   string
	reportType domain.ReportType
	help       bool
	rootHelp   bool
	version    bool
}

// Run 是可测试的进程入口。所有参数校验都先于文件写入。
func Run(args []string, stdout, stderr io.Writer) int {
	request, err := parseArgs(args)
	if err != nil {
		fmt.Fprintf(stderr, "%s 参数错误：%v\n", logPrefix, err)
		return int(domain.ExitUsage)
	}
	if request.rootHelp {
		fmt.Fprint(stdout, rootHelpText())
		return int(domain.ExitSuccess)
	}
	if request.help {
		fmt.Fprint(stdout, commandHelpText(request.name))
		return int(domain.ExitSuccess)
	}
	if request.version {
		fmt.Fprintf(stdout, "render-ai-news-page %s\n", Version)
		return int(domain.ExitSuccess)
	}

	switch request.name {
	case "build":
		return runBuild(request, stdout, stderr)
	case "lint":
		return runLint(request, stdout, stderr)
	default:
		fmt.Fprintf(stderr, "%s 参数错误：未知命令 %q\n", logPrefix, request.name)
		return int(domain.ExitUsage)
	}
}

func parseArgs(args []string) (command, error) {
	if len(args) == 0 {
		return command{}, errors.New("缺少命令；使用 --help 查看用法")
	}
	if args[0] == "--help" || args[0] == "-h" {
		if len(args) == 1 {
			return command{rootHelp: true}, nil
		}
		return command{}, controlArgumentError("帮助参数", args[1:])
	}
	if args[0] == "version" {
		if len(args) == 1 {
			return command{version: true}, nil
		}
		return command{}, controlArgumentError("version", args[1:])
	}
	if args[0] != "build" && args[0] != "lint" {
		return command{}, fmt.Errorf("未知命令 %q", args[0])
	}

	request := command{name: args[0]}
	seen := make(map[string]bool)
	businessArguments := 0
	for index := 1; index < len(args); {
		name := args[index]
		if name == "--help" || name == "-h" {
			if request.help {
				return command{}, errors.New("帮助参数不能重复")
			}
			request.help = true
			index++
			continue
		}
		allowed := name == "--source" || name == "--report-type" || (request.name == "build" && name == "--output") || (request.name == "lint" && name == "--page")
		if !allowed {
			if strings.HasPrefix(name, "-") {
				return command{}, fmt.Errorf("未知参数 %q", name)
			}
			return command{}, fmt.Errorf("不允许多余位置参数 %q", name)
		}
		if seen[name] {
			return command{}, fmt.Errorf("单值参数 %s 不能重复", name)
		}
		if index+1 >= len(args) || args[index+1] == "-h" || strings.HasPrefix(args[index+1], "--") {
			return command{}, fmt.Errorf("参数 %s 缺少值", name)
		}
		value := args[index+1]
		if value == "" {
			return command{}, fmt.Errorf("参数 %s 不能为空", name)
		}
		seen[name] = true
		switch name {
		case "--source":
			request.sourcePath = value
		case "--output":
			request.outputPath = value
		case "--page":
			request.pagePath = value
		case "--report-type":
			switch value {
			case string(domain.ReportDaily):
				request.reportType = domain.ReportDaily
			case string(domain.ReportWeekly):
				request.reportType = domain.ReportWeekly
			default:
				return command{}, fmt.Errorf("参数 --report-type 只允许 daily 或 weekly，实际为 %q", value)
			}
		}
		businessArguments++
		index += 2
	}
	if request.help {
		if businessArguments > 0 {
			return command{}, errors.New("帮助参数不能与业务参数混用")
		}
		return request, nil
	}
	if request.sourcePath == "" {
		return command{}, errors.New("缺少必填参数 --source")
	}
	if request.reportType == "" {
		return command{}, errors.New("缺少必填参数 --report-type；日报使用 daily，周报使用 weekly")
	}
	if request.name == "build" && request.outputPath == "" {
		return command{}, errors.New("缺少必填参数 --output")
	}
	if request.name == "lint" && request.pagePath == "" {
		return command{}, errors.New("缺少必填参数 --page")
	}
	return request, nil
}

// controlArgumentError 在进入帮助或版本分支前，优先报告尾随输入中的具体非法项。
func controlArgumentError(control string, trailing []string) error {
	for _, argument := range trailing {
		if argument == "--help" || argument == "-h" {
			return fmt.Errorf("%s不能重复或与其它参数混用", control)
		}
		if strings.HasPrefix(argument, "-") {
			return fmt.Errorf("未知参数 %q", argument)
		}
		return fmt.Errorf("%s不接受多余位置参数 %q", control, argument)
	}
	return fmt.Errorf("%s不接受其它参数", control)
}

func runBuild(request command, stdout, stderr io.Writer) int {
	sourcePath, sourceBytes, err := readInput(request.sourcePath, ".md", "新闻稿")
	if err != nil {
		return writeDomainError(stderr, err)
	}
	outputPath, err := validateOutputPath(request.outputPath, sourcePath)
	if err != nil {
		return writeDomainError(stderr, err)
	}

	pageText, source, issues := page.Render(sourceBytes, assets.NewsPageTemplate, request.reportType)
	if len(issues) == 0 {
		issues = page.LintRendered(pageText, source, assets.NewsPageTemplate)
	}
	if len(issues) > 0 {
		writeIssues(stderr, issues)
		return int(domain.ExitLint)
	}

	actualPath, writeErr := writeVerifiedAvoidingCollision(outputPath, []byte(pageText), func(written []byte) error {
		writtenText := strings.TrimPrefix(string(written), "\ufeff")
		writtenIssues := page.LintRendered(writtenText, source, assets.NewsPageTemplate)
		if len(writtenIssues) == 0 {
			return nil
		}
		return fmt.Errorf("落盘后 lint 失败：[%s] %s", writtenIssues[0].Code, writtenIssues[0].Message)
	})
	if writeErr != nil {
		return writeDomainError(stderr, &domain.Error{Code: domain.ExitIO, Message: "写入 HTML 失败", Cause: writeErr})
	}
	fmt.Fprintln(stdout, "状态: 成功")
	fmt.Fprintf(stdout, "报表类型: %s\n", request.reportType)
	fmt.Fprintf(stdout, "输出文件: %s\n", actualPath)
	fmt.Fprintf(stdout, "源文件 SHA-256: %s\n", source.SourceHash)
	fmt.Fprintf(stdout, "模板 SHA-256: %s\n", page.TemplateHash(assets.NewsPageTemplate))
	fmt.Fprintln(stdout, "lint: 通过")
	return int(domain.ExitSuccess)
}

func runLint(request command, stdout, stderr io.Writer) int {
	_, sourceBytes, err := readInput(request.sourcePath, ".md", "新闻稿")
	if err != nil {
		return writeDomainError(stderr, err)
	}
	pagePath, pageBytes, err := readInput(request.pagePath, ".html", "HTML 页面")
	if err != nil {
		return writeDomainError(stderr, err)
	}
	if !utf8.Valid(pageBytes) {
		writeIssues(stderr, []domain.Issue{{Code: "PAGE_UTF8", Message: "HTML 页面不是有效的 UTF-8"}})
		return int(domain.ExitLint)
	}

	issues := page.Lint(strings.TrimPrefix(string(pageBytes), "\ufeff"), sourceBytes, assets.NewsPageTemplate, request.reportType)
	if len(issues) > 0 {
		writeIssues(stderr, issues)
		return int(domain.ExitLint)
	}
	sourceDigest := sha256.Sum256(sourceBytes)
	sourceHash := fmt.Sprintf("%x", sourceDigest)
	fmt.Fprintln(stdout, "状态: 成功")
	fmt.Fprintf(stdout, "报表类型: %s\n", request.reportType)
	fmt.Fprintf(stdout, "页面: %s\n", pagePath)
	fmt.Fprintf(stdout, "源文件 SHA-256: %s\n", sourceHash)
	fmt.Fprintf(stdout, "模板 SHA-256: %s\n", page.TemplateHash(assets.NewsPageTemplate))
	fmt.Fprintln(stdout, "lint: 通过")
	return int(domain.ExitSuccess)
}

func readInput(rawPath, extension, label string) (string, []byte, *domain.Error) {
	absolute, err := filepath.Abs(rawPath)
	if err != nil {
		return "", nil, &domain.Error{Code: domain.ExitIO, Message: fmt.Sprintf("无法解析%s路径", label), Cause: err}
	}
	if !strings.EqualFold(filepath.Ext(absolute), extension) {
		return "", nil, &domain.Error{Code: domain.ExitUsage, Message: fmt.Sprintf("%s必须使用 %s 扩展名：%s", label, extension, absolute)}
	}
	info, err := os.Stat(absolute)
	if err != nil {
		return "", nil, &domain.Error{Code: domain.ExitIO, Message: fmt.Sprintf("%s不存在或不可读取：%s", label, absolute), Cause: err}
	}
	if !info.Mode().IsRegular() {
		return "", nil, &domain.Error{Code: domain.ExitUsage, Message: fmt.Sprintf("%s必须是普通文件：%s", label, absolute)}
	}
	content, err := os.ReadFile(absolute)
	if err != nil {
		return "", nil, &domain.Error{Code: domain.ExitIO, Message: fmt.Sprintf("读取%s失败：%s", label, absolute), Cause: err}
	}
	return absolute, content, nil
}

func validateOutputPath(rawPath, sourcePath string) (string, *domain.Error) {
	absolute, err := filepath.Abs(rawPath)
	if err != nil {
		return "", &domain.Error{Code: domain.ExitIO, Message: "无法解析输出路径", Cause: err}
	}
	if !strings.EqualFold(filepath.Ext(absolute), ".html") {
		return "", &domain.Error{Code: domain.ExitUsage, Message: fmt.Sprintf("输出文件必须使用 .html 扩展名：%s", absolute)}
	}
	if filepath.Clean(absolute) == filepath.Clean(sourcePath) {
		return "", &domain.Error{Code: domain.ExitUsage, Message: "输出路径不能与新闻稿路径相同"}
	}
	parent := filepath.Dir(absolute)
	info, err := os.Stat(parent)
	if err != nil {
		return "", &domain.Error{Code: domain.ExitIO, Message: fmt.Sprintf("输出目录不存在或不可访问：%s", parent), Cause: err}
	}
	if !info.IsDir() {
		return "", &domain.Error{Code: domain.ExitUsage, Message: fmt.Sprintf("输出文件的父路径不是目录：%s", parent)}
	}
	return absolute, nil
}

// writeVerifiedAvoidingCollision 只创建新文件，并在同步、回读和最终校验失败时清理本次候选文件。
func writeVerifiedAvoidingCollision(preferred string, content []byte, verify func([]byte) error) (string, error) {
	extension := filepath.Ext(preferred)
	base := strings.TrimSuffix(preferred, extension)
	for sequence := 1; ; sequence++ {
		candidate := preferred
		if sequence > 1 {
			candidate = fmt.Sprintf("%s-%d%s", base, sequence, extension)
		}
		file, err := os.OpenFile(candidate, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
		if errors.Is(err, os.ErrExist) {
			continue
		}
		if err != nil {
			return "", err
		}
		operationErr := error(nil)
		written, writeErr := file.Write(content)
		if writeErr != nil {
			operationErr = writeErr
		} else if written != len(content) {
			operationErr = io.ErrShortWrite
		}
		if operationErr == nil {
			if syncErr := file.Sync(); syncErr != nil {
				operationErr = fmt.Errorf("同步文件失败：%w", syncErr)
			}
		}
		if closeErr := file.Close(); closeErr != nil {
			operationErr = errors.Join(operationErr, fmt.Errorf("关闭文件失败：%w", closeErr))
		}
		if operationErr == nil {
			actual, readErr := os.ReadFile(candidate)
			if readErr != nil {
				operationErr = fmt.Errorf("回读文件失败：%w", readErr)
			} else if !bytes.Equal(actual, content) {
				operationErr = errors.New("回读内容与待写入 HTML 不一致")
			} else if verify != nil {
				operationErr = verify(actual)
			}
		}
		if operationErr != nil {
			if removeErr := os.Remove(candidate); removeErr != nil && !errors.Is(removeErr, os.ErrNotExist) {
				operationErr = errors.Join(operationErr, fmt.Errorf("清理失败，可能残留文件 %s：%w", candidate, removeErr))
			}
			return "", operationErr
		}
		return candidate, nil
	}
}

func writeIssues(writer io.Writer, issues []domain.Issue) {
	fmt.Fprintf(writer, "%s lint 失败：共 %d 个错误。\n", logPrefix, len(issues))
	shown := min(len(issues), issueCap)
	for index := 0; index < shown; index++ {
		fmt.Fprintf(writer, "%d. [%s] %s\n", index+1, issues[index].Code, issues[index].Message)
	}
	if len(issues) > shown {
		fmt.Fprintf(writer, "已省略 %d 个错误；lint 已扫描完整输入。\n", len(issues)-shown)
	}
}

func writeDomainError(writer io.Writer, err *domain.Error) int {
	fmt.Fprintf(writer, "%s 失败：%s\n", logPrefix, err.Error())
	return int(err.Code)
}

func rootHelpText() string {
	return `render-ai-news-page：将用户提供的新闻 Markdown 生成固定模板单页面 HTML。

用法：
  render-ai-news-page build --report-type <daily|weekly> --source <新闻.md> --output <目标.html>
  render-ai-news-page lint --report-type <daily|weekly> --source <新闻.md> --page <页面.html>
  render-ai-news-page version
  render-ai-news-page --help

--report-type 必填；daily 与 weekly 分别执行独立的日报和周报规则。
目标已存在时 build 自动使用 -2、-3 后缀避让，不覆盖已有文件。
`
}

func commandHelpText(name string) string {
	if name == "build" {
		return `用法：render-ai-news-page build --report-type <daily|weekly> --source <新闻.md> --output <目标.html>

--report-type  必填，daily 使用日报规则，weekly 使用周报规则，只能出现一次。
--source  必填，UTF-8 Markdown 新闻稿，只能出现一次。
--output  必填，目标 .html 路径，只能出现一次；父目录必须存在。
`
	}
	return `用法：render-ai-news-page lint --report-type <daily|weekly> --source <新闻.md> --page <页面.html>

--report-type  必填，必须与构建页面时使用的日报或周报规则一致，只能出现一次。
--source  必填，生成时使用的 UTF-8 Markdown 新闻稿，只能出现一次。
--page    必填，待校验的 .html 页面，只能出现一次。
`
}
