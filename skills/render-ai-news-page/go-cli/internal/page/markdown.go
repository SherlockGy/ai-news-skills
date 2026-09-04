package page

import (
	"bytes"
	"fmt"
	"net/url"
	"regexp"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/SherlockGy/ai-news-skills/skills/render-ai-news-page/go-cli/internal/domain"
	cjkfriendly "github.com/tats-u/goldmark-cjk-friendly/v2"
	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/extension"
	goldhtml "github.com/yuin/goldmark/renderer/html"
	"github.com/yuin/goldmark/text"
)

const (
	dailyThoughtHighlight  = "今日思想亮点"
	weeklyThoughtHighlight = "本周判断"
)

var (
	datePattern      = regexp.MustCompile(`^生成(?:日期|时间)\s*[：:]\s*(\d{4})\s*年\s*(\d{1,2})\s*月\s*(\d{1,2})\s*日`)
	dateRangePattern = regexp.MustCompile(
		`^(?:日期范围|统计周期)\s*[：:]\s*` +
			`(\d{4})\s*年\s*(\d{1,2})\s*月\s*(\d{1,2})\s*日` +
			`(?:\s*[（(][^（）()\r\n]*[）)])?\s*` +
			`(?:至|到|—|–|-|～|~)\s*` +
			`(?:(\d{4})\s*年\s*)?(\d{1,2})\s*月\s*(\d{1,2})\s*日`,
	)
	manualBreakPattern = regexp.MustCompile(`(?i)<\s*(?:br|wbr)\b`)
)

var forbiddenHeadingCharacters = map[rune]string{
	'\u00a0': "不换行空格",
	'\u200b': "零宽空格",
	'\u2060': "单词连接符",
	'\ufeff': "零宽不换行空格",
}

// SourceDocument 保存从新闻 Markdown 中提取的、生成与校验共同使用的事实。
type SourceDocument struct {
	HTML        string
	Title       string
	ReportType  domain.ReportType
	PeriodStart time.Time
	PeriodEnd   time.Time
	H1Titles    []string
	H2Titles    []string
	SourceHash  string
}

func newMarkdown() goldmark.Markdown {
	return goldmark.New(
		goldmark.WithExtensions(
			extension.Table,
			cjkfriendly.CJKFriendlyEmphasisAndStrikethrough,
		),
		goldmark.WithRendererOptions(goldhtml.WithHardWraps()),
	)
}

// ParseSource 完整解析用户提供的新闻稿，并返回所有可确认的结构事实。
func ParseSource(sourceBytes []byte, reportType domain.ReportType) (SourceDocument, []domain.Issue) {
	if reportType != domain.ReportDaily && reportType != domain.ReportWeekly {
		return SourceDocument{ReportType: reportType}, []domain.Issue{{
			Code:    "SOURCE_REPORT_TYPE",
			Message: fmt.Sprintf("不支持的报表类型：%q；只允许 daily 或 weekly", reportType),
		}}
	}

	issues := make([]domain.Issue, 0)
	if !utf8.Valid(sourceBytes) {
		return SourceDocument{}, []domain.Issue{{Code: "SOURCE_UTF8", Message: "新闻 Markdown 不是有效的 UTF-8"}}
	}

	textValue := strings.TrimPrefix(string(sourceBytes), "\ufeff")
	if strings.TrimSpace(textValue) == "" {
		return SourceDocument{}, []domain.Issue{{Code: "SOURCE_EMPTY", Message: "新闻 Markdown 为空"}}
	}
	if strings.Contains(textValue, "{{") || strings.Contains(textValue, "}}") {
		issues = append(issues, domain.Issue{Code: "SOURCE_PLACEHOLDER", Message: "新闻 Markdown 中仍有模板占位符"})
	}

	markdown := newMarkdown()
	reader := text.NewReader([]byte(textValue))
	document := markdown.Parser().Parse(reader)
	first := document.FirstChild()
	if first == nil || first.Kind() != ast.KindHeading || first.(*ast.Heading).Level != 1 {
		issues = append(issues, domain.Issue{Code: "SOURCE_MASTHEAD", Message: "第一块内容必须是非空一级报刊标题"})
	}

	title := ""
	h1Titles := make([]string, 0)
	h2Titles := make([]string, 0)
	hasRawHTML := false
	_ = ast.Walk(document, func(node ast.Node, entering bool) (ast.WalkStatus, error) {
		if !entering {
			return ast.WalkContinue, nil
		}
		switch typed := node.(type) {
		case *ast.Heading:
			heading := strings.TrimSpace(string(typed.Text([]byte(textValue))))
			if typed.Level == 1 {
				h1Titles = append(h1Titles, heading)
				if title == "" {
					title = heading
				}
			}
			if typed.Level == 2 {
				h2Titles = append(h2Titles, heading)
			}
			validateHeading(typed.Level, heading, &issues)
		case *ast.Link:
			validateLink(string(typed.Destination), &issues)
		case *ast.AutoLink:
			validateLink(string(typed.URL([]byte(textValue))), &issues)
		case *ast.Image:
			validateLink(string(typed.Destination), &issues)
		case *ast.RawHTML, *ast.HTMLBlock:
			hasRawHTML = true
		}
		return ast.WalkContinue, nil
	})

	if title == "" {
		issues = append(issues, domain.Issue{Code: "SOURCE_MASTHEAD", Message: "一级报刊标题不能为空"})
	}
	issues = append(issues, validateReportStructure(reportType, h1Titles, h2Titles)...)
	if hasRawHTML {
		issues = append(issues, domain.Issue{Code: "SOURCE_RAW_HTML", Message: "新闻 Markdown 不允许包含原始 HTML"})
	}

	dateText := sourcePeriodText(first, []byte(textValue), reportType)
	periodStart, periodEnd, periodIssue := extractReportPeriod(dateText, reportType)
	if periodIssue != nil {
		issues = append(issues, *periodIssue)
	}

	var rendered bytes.Buffer
	if err := markdown.Renderer().Render(&rendered, []byte(textValue), document); err != nil {
		issues = append(issues, domain.Issue{Code: "SOURCE_RENDER", Message: fmt.Sprintf("Markdown 渲染失败：%v", err)})
	}

	return SourceDocument{
		HTML:        strings.TrimSpace(rendered.String()),
		Title:       title,
		ReportType:  reportType,
		PeriodStart: periodStart,
		PeriodEnd:   periodEnd,
		H1Titles:    h1Titles,
		H2Titles:    h2Titles,
	}, deduplicateIssues(issues)
}

func validateReportStructure(reportType domain.ReportType, h1Titles, h2Titles []string) []domain.Issue {
	issues := make([]domain.Issue, 0)
	switch reportType {
	case domain.ReportDaily:
		if len(h1Titles) < 2 {
			issues = append(issues, domain.Issue{Code: "SOURCE_SECTIONS", Message: "日报的报刊标题后至少需要一个一级章节"})
		}
		if len(h2Titles) == 0 || h2Titles[0] != dailyThoughtHighlight {
			issues = append(issues, domain.Issue{Code: "SOURCE_HIGHLIGHT", Message: "日报报刊标题后的第一个二级标题必须是“今日思想亮点”"})
		}
	case domain.ReportWeekly:
		if len(h1Titles) != 1 {
			issues = append(issues, domain.Issue{Code: "SOURCE_WEEKLY_SECTIONS", Message: "周报只能使用首个一级标题作为报刊标题，正文主章节使用二级标题"})
		}
		if len(h2Titles) == 0 || h2Titles[0] != weeklyThoughtHighlight {
			issues = append(issues, domain.Issue{Code: "SOURCE_WEEKLY_HIGHLIGHT", Message: "周报报刊标题后的第一个二级标题必须是“本周判断”"})
		}
		if len(h2Titles) < 2 {
			issues = append(issues, domain.Issue{Code: "SOURCE_WEEKLY_SECTIONS", Message: "周报在“本周判断”后至少需要一个二级正文章节"})
		}
	}
	return issues
}

// sourcePeriodText 只读取报刊标题后的日期块，避免正文中的日期误通过校验。
func sourcePeriodText(masthead ast.Node, source []byte, reportType domain.ReportType) string {
	if masthead == nil || masthead.NextSibling() == nil {
		return ""
	}
	periodNode := masthead.NextSibling()
	if periodNode.Kind() == ast.KindParagraph {
		return normalizeMarkdownPeriodText(string(periodNode.Text(source)))
	}
	if reportType == domain.ReportWeekly && periodNode.Kind() == ast.KindBlockquote {
		for child := periodNode.FirstChild(); child != nil; child = child.NextSibling() {
			if child.Kind() == ast.KindParagraph {
				return normalizeMarkdownPeriodText(string(child.Text(source)))
			}
		}
	}
	return ""
}

func normalizeMarkdownPeriodText(value string) string {
	normalized := strings.TrimSpace(value)
	for _, marker := range []string{"**", "__"} {
		normalized = strings.TrimPrefix(normalized, marker)
	}
	return strings.TrimSpace(normalized)
}

func extractReportPeriod(source string, reportType domain.ReportType) (time.Time, time.Time, *domain.Issue) {
	if reportType == domain.ReportWeekly {
		return extractDateRange(source)
	}
	dateValue, issue := extractDate(source)
	return dateValue, dateValue, issue
}

func extractDate(source string) (time.Time, *domain.Issue) {
	matches := datePattern.FindStringSubmatch(source)
	if len(matches) != 4 {
		return time.Time{}, &domain.Issue{Code: "SOURCE_DATE", Message: "新闻 Markdown 必须包含“生成日期：”或“生成时间：”及有效中文日期"}
	}
	parsed, err := time.Parse("2006-1-2", fmt.Sprintf("%s-%s-%s", matches[1], matches[2], matches[3]))
	if err != nil {
		return time.Time{}, &domain.Issue{Code: "SOURCE_DATE", Message: "新闻 Markdown 中的生成日期无效"}
	}
	return parsed, nil
}

func extractDateRange(source string) (time.Time, time.Time, *domain.Issue) {
	matches := dateRangePattern.FindStringSubmatch(strings.TrimSpace(source))
	if len(matches) != 7 {
		return time.Time{}, time.Time{}, &domain.Issue{Code: "SOURCE_DATE_RANGE", Message: "周报 Markdown 必须在报刊标题后包含“日期范围：”或“统计周期：”及有效起止日期"}
	}
	start, err := time.Parse("2006-1-2", fmt.Sprintf("%s-%s-%s", matches[1], matches[2], matches[3]))
	if err != nil {
		return time.Time{}, time.Time{}, &domain.Issue{Code: "SOURCE_DATE_RANGE", Message: "周报 Markdown 中的开始日期无效"}
	}
	endYear := matches[4]
	if endYear == "" {
		endYear = matches[1]
	}
	end, err := time.Parse("2006-1-2", fmt.Sprintf("%s-%s-%s", endYear, matches[5], matches[6]))
	if err != nil {
		return time.Time{}, time.Time{}, &domain.Issue{Code: "SOURCE_DATE_RANGE", Message: "周报 Markdown 中的结束日期无效"}
	}
	if end.Before(start) {
		return time.Time{}, time.Time{}, &domain.Issue{Code: "SOURCE_DATE_RANGE", Message: "周报日期范围的结束日期不能早于开始日期"}
	}
	return start, end, nil
}

func validateHeading(level int, heading string, issues *[]domain.Issue) {
	if heading == "" {
		*issues = append(*issues, domain.Issue{Code: "SOURCE_HEADING_EMPTY", Message: fmt.Sprintf("存在空的 h%d 标题", level)})
		return
	}
	for character, label := range forbiddenHeadingCharacters {
		if strings.ContainsRune(heading, character) {
			*issues = append(*issues, domain.Issue{Code: "SOURCE_HEADING_BREAK", Message: fmt.Sprintf("标题“%s”包含%s", heading, label)})
		}
	}
	if manualBreakPattern.MatchString(heading) {
		*issues = append(*issues, domain.Issue{Code: "SOURCE_HEADING_BREAK", Message: fmt.Sprintf("标题“%s”包含手工换行标签", heading)})
	}
}

func validateLink(raw string, issues *[]domain.Issue) {
	parsed, err := url.Parse(raw)
	if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.Host == "" {
		*issues = append(*issues, domain.Issue{Code: "SOURCE_LINK", Message: fmt.Sprintf("来源链接必须是完整的 HTTP(S) 地址：%s", raw)})
	}
}

func deduplicateIssues(issues []domain.Issue) []domain.Issue {
	result := make([]domain.Issue, 0, len(issues))
	seen := make(map[string]struct{}, len(issues))
	for _, issue := range issues {
		key := issue.Code + "\x00" + issue.Message
		if _, exists := seen[key]; exists {
			continue
		}
		seen[key] = struct{}{}
		result = append(result, issue)
	}
	return result
}
