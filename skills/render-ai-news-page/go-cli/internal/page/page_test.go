package page

import (
	"strings"
	"testing"

	"finance-skills/common/render-ai-news-page/go-cli/assets"
	"finance-skills/common/render-ai-news-page/go-cli/internal/domain"
)

const validSource = `# 企业 AI 工作变革速览

**生成时间：2026 年 8 月 13 日 06:00（北京时间）**
**主要观察窗口：过去 24 小时。**

今天的重点是：**Agent 正进入企业工作流。**今天仍需观察经营结果。

## 今日思想亮点

**为什么值得关注：**这是一个中文紧邻加粗的回归案例。

# 今日企业 AI 重要动态

## 1. 一个用于验证自然换行的较长新闻标题：保留限定信息和完整产品名称

来源见 [官方页面](https://example.com/news "官方新闻页面")。

` + "```text\n**代码中的 Markdown 标记必须按原样保留**\n```" + `
`

const validWeeklySource = `# AI 科技周报｜能力竞赛进入安全、算力与治理约束期

> **统计周期：2026 年 8 月 15 日（周六）至 8 月 21 日（周五）**
>
> 信息截点：北京时间 2026 年 8 月 21 日 11:05。

## 本周判断

本周的重点判断。

## 重点新闻排序

### 1. 用于验证周报结构的新闻标题

正文及 [来源](https://example.com/weekly)。
`

func TestRenderAndLintValidPage(t *testing.T) {
	t.Parallel()

	pageText, source, issues := Render([]byte(validSource), assets.NewsPageTemplate, domain.ReportDaily)
	if len(issues) != 0 {
		t.Fatalf("Render() issues = %#v", issues)
	}
	if !strings.Contains(pageText, "<strong>Agent 正进入企业工作流。</strong>今天") {
		t.Fatalf("中文紧邻加粗未正确渲染：%s", pageText)
	}
	if !strings.Contains(pageText, "<strong>为什么值得关注：</strong>这是") {
		t.Fatalf("中文标签加粗未正确渲染：%s", pageText)
	}
	if source.SourceHash == "" {
		t.Fatal("源文件哈希为空")
	}
	if issues = Lint(pageText, []byte(validSource), assets.NewsPageTemplate, domain.ReportDaily); len(issues) != 0 {
		t.Fatalf("Lint() issues = %#v", issues)
	}
}

func TestLintDetectsPageAndHeadingRuleDrift(t *testing.T) {
	t.Parallel()

	pageText, _, issues := Render([]byte(validSource), assets.NewsPageTemplate, domain.ReportDaily)
	if len(issues) != 0 {
		t.Fatalf("Render() issues = %#v", issues)
	}
	tampered := strings.Replace(pageText, "Agent 正进入企业工作流", "Agent 被手工改写", 1)
	issues = Lint(tampered, []byte(validSource), assets.NewsPageTemplate, domain.ReportDaily)
	assertIssueCode(t, issues, "PAGE_NOT_REPRODUCIBLE")

	badTemplate := strings.Replace(assets.NewsPageTemplate, "text-wrap: pretty", "text-wrap: balance", 1)
	badPage, _, renderIssues := Render([]byte(validSource), badTemplate, domain.ReportDaily)
	if len(renderIssues) != 0 {
		t.Fatalf("Render(badTemplate) issues = %#v", renderIssues)
	}
	issues = Lint(badPage, []byte(validSource), badTemplate, domain.ReportDaily)
	assertIssueCode(t, issues, "CSS_HEADING_BALANCE")
	assertIssueCode(t, issues, "CSS_HEADING_RULE")

	badWordCSS := strings.Replace(assets.NewsPageTemplate, ".article .heading-word { white-space: nowrap; }", ".article .heading-word { white-space: normal; }", 1)
	badPage, _, renderIssues = Render([]byte(validSource), badWordCSS, domain.ReportDaily)
	if len(renderIssues) != 0 {
		t.Fatalf("Render(badWordCSS) issues = %#v", renderIssues)
	}
	issues = Lint(badPage, []byte(validSource), badWordCSS, domain.ReportDaily)
	assertIssueCode(t, issues, "CSS_HEADING_WORD")

	badBreakCSS := strings.Replace(assets.NewsPageTemplate, ".article .heading-break { display: block; height: 0; }", ".article .heading-break { display: inline; height: auto; }", 1)
	badPage, _, renderIssues = Render([]byte(validSource), badBreakCSS, domain.ReportDaily)
	if len(renderIssues) != 0 {
		t.Fatalf("Render(badBreakCSS) issues = %#v", renderIssues)
	}
	issues = Lint(badPage, []byte(validSource), badBreakCSS, domain.ReportDaily)
	assertIssueCode(t, issues, "CSS_HEADING_BREAK")

	badWordScript := strings.Replace(assets.NewsPageTemplate, "safely('标题智能换行'", "safely('标题布局'", 1)
	badPage, _, renderIssues = Render([]byte(validSource), badWordScript, domain.ReportDaily)
	if len(renderIssues) != 0 {
		t.Fatalf("Render(badWordScript) issues = %#v", renderIssues)
	}
	issues = Lint(badPage, []byte(validSource), badWordScript, domain.ReportDaily)
	assertIssueCode(t, issues, "JS_HEADING_WRAP")
}

func TestHeadingWrapUsesSemanticCandidatesWithoutFixedLengthGrouping(t *testing.T) {
	t.Parallel()

	template := assets.NewsPageTemplate
	for _, marker := range []string{
		"new Intl.Segmenter('zh-CN'",
		"strongBreakPunctuation",
		"boundaryPenalty",
		"noBreakGroups",
		"lineLimit",
		"heading.clientWidth",
		"document.fonts.ready",
		"setTimeout(() => headings.forEach(layoutHeading), 0)",
		"relayoutHeadings();",
	} {
		if !strings.Contains(template, marker) {
			t.Fatalf("标题换行实现缺少必要标记：%s", marker)
		}
	}
	if strings.Contains(template, "<= 8") || strings.Contains(template, "3～8 字") {
		t.Fatal("标题换行仍包含固定字数拼块逻辑")
	}
}

func TestTemplateRecognizesDailyAndWeeklyDateBlocks(t *testing.T) {
	t.Parallel()

	template := assets.NewsPageTemplate
	for _, marker := range []string{
		"first.tagName === 'P' || first.tagName === 'BLOCKQUOTE'",
		"生成日期|生成时间|日期范围|统计周期",
		"p.closest('.date-plate')",
		"blockquote.date-plate > p",
	} {
		if !strings.Contains(template, marker) {
			t.Fatalf("日报/周报日期牌实现缺少必要标记：%s", marker)
		}
	}
}

func TestParseSourceRejectsInvalidStructureAndRawHTML(t *testing.T) {
	t.Parallel()

	invalid := `# 标题

这里不是生成日期段。

导语中提到了生成时间：2026 年 8 月 13 日，但它不在报头后的日期段。

## 今日思想亮点

<br>

# 章节
`
	_, issues := ParseSource([]byte(invalid), domain.ReportDaily)
	assertIssueCode(t, issues, "SOURCE_DATE")
	assertIssueCode(t, issues, "SOURCE_RAW_HTML")
}

func TestRenderAndLintWeeklyPage(t *testing.T) {
	t.Parallel()

	pageText, source, issues := Render([]byte(validWeeklySource), assets.NewsPageTemplate, domain.ReportWeekly)
	if len(issues) != 0 {
		t.Fatalf("Render(weekly) issues = %#v", issues)
	}
	if source.ReportType != domain.ReportWeekly || source.PeriodStart.Format("2006-01-02") != "2026-08-15" || source.PeriodEnd.Format("2006-01-02") != "2026-08-21" {
		t.Fatalf("周报日期范围解析错误：%#v", source)
	}
	if !strings.Contains(pageText, "2026.08.15–2026.08.21") {
		t.Fatalf("周报页面 title 未包含完整日期范围：%s", pageText)
	}
	if !strings.Contains(pageText, "2026 年 8 月 15 日—2026 年 8 月 21 日") {
		t.Fatalf("周报页面描述未包含完整日期范围：%s", pageText)
	}
	if issues = Lint(pageText, []byte(validWeeklySource), assets.NewsPageTemplate, domain.ReportWeekly); len(issues) != 0 {
		t.Fatalf("Lint(weekly) issues = %#v", issues)
	}
}

func TestReportTypesUseIndependentRules(t *testing.T) {
	t.Parallel()

	_, dailyAsWeeklyIssues := ParseSource([]byte(validSource), domain.ReportWeekly)
	assertIssueCode(t, dailyAsWeeklyIssues, "SOURCE_DATE_RANGE")
	assertIssueCode(t, dailyAsWeeklyIssues, "SOURCE_WEEKLY_HIGHLIGHT")

	_, weeklyAsDailyIssues := ParseSource([]byte(validWeeklySource), domain.ReportDaily)
	assertIssueCode(t, weeklyAsDailyIssues, "SOURCE_DATE")
	assertIssueCode(t, weeklyAsDailyIssues, "SOURCE_SECTIONS")
	assertIssueCode(t, weeklyAsDailyIssues, "SOURCE_HIGHLIGHT")
}

func TestRenderRejectsUnknownReportType(t *testing.T) {
	t.Parallel()

	pageText, _, issues := Render([]byte(validSource), assets.NewsPageTemplate, domain.ReportType("monthly"))
	if pageText != "" {
		t.Fatalf("非法报表类型仍生成页面：%s", pageText)
	}
	assertIssueCode(t, issues, "SOURCE_REPORT_TYPE")
}

func TestWeeklyDateRangeValidation(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		header    string
		wantStart string
		wantEnd   string
		wantIssue bool
	}{
		{name: "任意星期且同年省略结束年份", header: "统计周期：2026 年 8 月 18 日（周二）至 8 月 25 日（周二）", wantStart: "2026-08-18", wantEnd: "2026-08-25"},
		{name: "跨月", header: "日期范围：2026 年 8 月 30 日至 2026 年 9 月 5 日", wantStart: "2026-08-30", wantEnd: "2026-09-05"},
		{name: "跨年", header: "日期范围：2026 年 12 月 29 日—2027 年 1 月 4 日", wantStart: "2026-12-29", wantEnd: "2027-01-04"},
		{name: "反向日期", header: "日期范围：2026 年 8 月 21 日至 8 月 15 日", wantIssue: true},
		{name: "缺少结束日期", header: "日期范围：2026 年 8 月 15 日", wantIssue: true},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			start, end, issue := extractDateRange(test.header)
			if test.wantIssue {
				if issue == nil {
					t.Fatalf("extractDateRange(%q) 未返回错误", test.header)
				}
				return
			}
			if issue != nil {
				t.Fatalf("extractDateRange(%q) issue = %v", test.header, issue)
			}
			if start.Format("2006-01-02") != test.wantStart || end.Format("2006-01-02") != test.wantEnd {
				t.Fatalf("extractDateRange(%q) = %s 至 %s", test.header, start.Format("2006-01-02"), end.Format("2006-01-02"))
			}
		})
	}
}

func TestTemplateTokenContract(t *testing.T) {
	t.Parallel()

	for token, expected := range templateTokens {
		if actual := strings.Count(assets.NewsPageTemplate, token); actual != expected {
			t.Fatalf("模板占位符 %s 数量 = %d，期望 %d", token, actual, expected)
		}
	}
	if issues := validateTemplate(assets.NewsPageTemplate); len(issues) != 0 {
		t.Fatalf("validateTemplate() issues = %#v", issues)
	}
}

func assertIssueCode(t *testing.T, issues []domain.Issue, expected string) {
	t.Helper()
	for _, issue := range issues {
		if issue.Code == expected {
			return
		}
	}
	t.Fatalf("未找到错误码 %s：%#v", expected, issues)
}
