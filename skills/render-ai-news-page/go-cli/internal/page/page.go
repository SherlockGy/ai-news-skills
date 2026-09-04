// Package page 完成新闻 Markdown 渲染、模板装配与确定性 lint。
package page

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"html"
	"io"
	"net/url"
	"regexp"
	"strings"

	"github.com/SherlockGy/ai-news-skills/skills/render-ai-news-page/go-cli/internal/domain"
	"github.com/dop251/goja/parser"
	parse "github.com/tdewolff/parse/v2"
	cssparser "github.com/tdewolff/parse/v2/css"
	nethtml "golang.org/x/net/html"
)

var templateTokens = map[string]int{
	"{{DATE_CN}}":         1,
	"{{DATE_DOTTED}}":     1,
	"{{REPORT_TITLE}}":    2,
	"{{SOURCE_SHA256}}":   1,
	"{{TEMPLATE_SHA256}}": 1,
	"{{ARTICLE_HTML}}":    1,
}

var (
	placeholderPattern      = regexp.MustCompile(`\{\{[^{}]+\}\}`)
	markdownStrongMark      = regexp.MustCompile(`\*\*[^*\n]+\*\*`)
	headingRulePattern      = regexp.MustCompile(`(?s)\.article\s+h1\s*,\s*\.article\s+h2\s*,\s*\.article\s+h3\s*\{([^}]*)\}`)
	headingWordRulePattern  = regexp.MustCompile(`(?s)\.article\s+\.heading-word\s*\{([^}]*)\}`)
	headingBreakRulePattern = regexp.MustCompile(`(?s)\.article\s+\.heading-break\s*\{([^}]*)\}`)
)

// Render 使用唯一模板生成候选页面。所有 lint 通过后调用方才可以落盘。
func Render(sourceBytes []byte, templateText string, reportType domain.ReportType) (string, SourceDocument, []domain.Issue) {
	document, issues := ParseSource(sourceBytes, reportType)
	issues = append(issues, validateTemplate(templateText)...)
	if len(issues) > 0 {
		return "", document, deduplicateIssues(issues)
	}

	document.SourceHash = hashBytes(sourceBytes)
	templateHash := hashTemplate(templateText)
	dateCN, dateDotted := formatReportPeriod(document)
	replacements := map[string]string{
		"{{DATE_CN}}":         dateCN,
		"{{DATE_DOTTED}}":     dateDotted,
		"{{REPORT_TITLE}}":    html.EscapeString(document.Title),
		"{{SOURCE_SHA256}}":   document.SourceHash,
		"{{TEMPLATE_SHA256}}": templateHash,
		"{{ARTICLE_HTML}}":    document.HTML,
	}

	pageText := templateText
	for token, value := range replacements {
		pageText = strings.ReplaceAll(pageText, token, value)
	}
	pageText = strings.TrimRight(pageText, "\r\n") + "\n"
	return pageText, document, nil
}

// Lint 验证页面可由当前模板和完整源稿确定性重建，并检查页面结构与脚本样式。
func Lint(pageText string, sourceBytes []byte, templateText string, reportType domain.ReportType) []domain.Issue {
	expected, source, issues := Render(sourceBytes, templateText, reportType)
	if len(issues) == 0 && pageText != expected {
		issues = append(issues, domain.Issue{
			Code:    "PAGE_NOT_REPRODUCIBLE",
			Message: "HTML 与当前模板和新闻稿的确定性构建结果不一致，差异位于" + firstDifference(pageText, expected),
		})
	}
	issues = append(issues, LintRendered(pageText, source, templateText)...)
	return deduplicateIssues(issues)
}

// LintRendered 检查已经完成渲染的候选页面，不重复解析或渲染新闻 Markdown。
func LintRendered(pageText string, source SourceDocument, templateText string) []domain.Issue {
	issues := make([]domain.Issue, 0)
	if placeholderPattern.MatchString(pageText) {
		issues = append(issues, domain.Issue{Code: "PAGE_PLACEHOLDER", Message: "HTML 中仍有未替换的模板占位符"})
	}
	if !strings.HasPrefix(strings.ToLower(strings.TrimSpace(pageText)), "<!doctype html>") {
		issues = append(issues, domain.Issue{Code: "HTML5_DOCTYPE", Message: "页面必须以 <!doctype html> 开头"})
	}

	root, err := nethtml.Parse(strings.NewReader(pageText))
	if err != nil {
		issues = append(issues, domain.Issue{Code: "HTML5_SYNTAX", Message: fmt.Sprintf("HTML5 解析失败：%v", err)})
		return deduplicateIssues(issues)
	}
	htmlRoots := elements(root, "html")
	if len(htmlRoots) != 1 || attribute(htmlRoots[0], "lang") != "zh-CN" {
		issues = append(issues, domain.Issue{Code: "HTML5_ROOT", Message: "页面必须包含唯一的 <html lang=\"zh-CN\"> 根元素"})
	}
	if len(elements(root, "head")) != 1 || len(elements(root, "body")) != 1 || len(elements(root, "main")) != 1 {
		issues = append(issues, domain.Issue{Code: "HTML5_STRUCTURE", Message: "页面必须包含唯一的 head、body 和 main"})
	}

	dateCN, dateDotted := formatReportPeriod(source)
	issues = append(issues, lintHead(root, source, dateCN, dateDotted, templateText)...)
	issues = append(issues, lintArticle(root, source, dateCN)...)
	issues = append(issues, lintStylesAndScripts(root)...)
	return deduplicateIssues(issues)
}

func validateTemplate(templateText string) []domain.Issue {
	issues := make([]domain.Issue, 0)
	for token, expectedCount := range templateTokens {
		if actual := strings.Count(templateText, token); actual != expectedCount {
			issues = append(issues, domain.Issue{Code: "TEMPLATE_TOKEN", Message: fmt.Sprintf("模板占位符 %s 应出现 %d 次，实际出现 %d 次", token, expectedCount, actual)})
		}
	}
	found := placeholderPattern.FindAllString(templateText, -1)
	for _, token := range found {
		if _, allowed := templateTokens[token]; !allowed {
			issues = append(issues, domain.Issue{Code: "TEMPLATE_TOKEN", Message: fmt.Sprintf("模板包含未知占位符：%s", token)})
		}
	}
	return issues
}

func lintHead(root *nethtml.Node, source SourceDocument, dateCN, dateDotted, templateText string) []domain.Issue {
	issues := make([]domain.Issue, 0)
	titles := elements(root, "title")
	expectedTitle := source.Title + "｜" + dateDotted
	if len(titles) != 1 || nodeText(titles[0]) != expectedTitle {
		issues = append(issues, domain.Issue{Code: "PAGE_TITLE", Message: fmt.Sprintf("页面 title 必须是“%s”", expectedTitle)})
	}

	metaValues := make(map[string]string)
	for _, meta := range elements(root, "meta") {
		name := attribute(meta, "name")
		if name != "" {
			metaValues[name] = attribute(meta, "content")
		}
	}
	expectedDescription := fmt.Sprintf("%s %s：完整正文与原始来源。", dateCN, source.Title)
	expectMeta(&issues, metaValues, "description", expectedDescription, "PAGE_DESCRIPTION")
	expectMeta(&issues, metaValues, "generator", "render-ai-news-page", "PAGE_GENERATOR")
	expectMeta(&issues, metaValues, "news-source-sha256", source.SourceHash, "SOURCE_HASH")
	expectMeta(&issues, metaValues, "news-template-sha256", hashTemplate(templateText), "TEMPLATE_HASH")
	return issues
}

func lintArticle(root *nethtml.Node, source SourceDocument, dateCN string) []domain.Issue {
	issues := make([]domain.Issue, 0)
	articles := make([]*nethtml.Node, 0)
	for _, article := range elements(root, "article") {
		if hasClass(article, "article") {
			articles = append(articles, article)
		}
	}
	if len(articles) != 1 {
		return []domain.Issue{{Code: "ARTICLE_COUNT", Message: "页面必须且只能包含一个 article.article"}}
	}
	article := articles[0]
	children := elementChildren(article)
	if len(children) < 3 {
		issues = append(issues, domain.Issue{Code: "ARTICLE_HEADER", Message: "正文至少需要报头、日期和后续正文三个块"})
	} else {
		if children[0].Data != "h1" || nodeText(children[0]) != source.Title {
			issues = append(issues, domain.Issue{Code: "ARTICLE_MASTHEAD", Message: fmt.Sprintf("正文首个元素必须是 h1“%s”", source.Title)})
		}
		issues = append(issues, lintArticlePeriod(children[1], source, dateCN)...)
	}

	actualH1 := headingTexts(article, "h1")
	if !equalStrings(actualH1, source.H1Titles) {
		issues = append(issues, domain.Issue{Code: "ARTICLE_SECTIONS", Message: "生成页的一级标题与新闻 Markdown 不一致"})
	}
	actualH2 := headingTexts(article, "h2")
	expectedHighlight := dailyThoughtHighlight
	highlightCode := "ARTICLE_HIGHLIGHT"
	if source.ReportType == domain.ReportWeekly {
		expectedHighlight = weeklyThoughtHighlight
		highlightCode = "ARTICLE_WEEKLY_HIGHLIGHT"
	}
	if len(actualH2) == 0 || actualH2[0] != expectedHighlight {
		issues = append(issues, domain.Issue{Code: highlightCode, Message: fmt.Sprintf("%s首个二级标题必须是“%s”", reportTypeName(source.ReportType), expectedHighlight)})
	}

	forbidden := map[string]bool{"script": true, "style": true, "iframe": true, "object": true, "embed": true, "form": true, "input": true, "button": true}
	walkNodes(article, func(node *nethtml.Node) {
		if node.Type != nethtml.ElementNode {
			return
		}
		if forbidden[node.Data] {
			issues = append(issues, domain.Issue{Code: "ARTICLE_UNSAFE_TAG", Message: fmt.Sprintf("正文不允许出现 <%s>", node.Data)})
		}
		if node.Data == "h1" || node.Data == "h2" || node.Data == "h3" || node.Data == "h4" {
			heading := nodeText(node)
			if len(elements(node, "br")) > 0 || len(elements(node, "wbr")) > 0 {
				issues = append(issues, domain.Issue{Code: "HEADING_MANUAL_BREAK", Message: fmt.Sprintf("标题“%s”包含手工换行元素", heading)})
			}
			for character, label := range forbiddenHeadingCharacters {
				if strings.ContainsRune(heading, character) {
					issues = append(issues, domain.Issue{Code: "HEADING_MANUAL_BREAK", Message: fmt.Sprintf("标题“%s”包含%s", heading, label)})
				}
			}
		}
		if node.Data == "a" {
			href := attribute(node, "href")
			parsed, err := url.Parse(href)
			if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.Host == "" {
				issues = append(issues, domain.Issue{Code: "ARTICLE_LINK", Message: fmt.Sprintf("正文链接不是完整的 HTTP(S) 地址：%s", href)})
			}
			if strings.TrimSpace(nodeText(node)) == "" {
				issues = append(issues, domain.Issue{Code: "ARTICLE_LINK_TEXT", Message: fmt.Sprintf("正文链接缺少可见文字：%s", href)})
			}
		}
	})

	visibleText := textWithoutCode(article)
	if markdownStrongMark.MatchString(visibleText) {
		issues = append(issues, domain.Issue{Code: "ARTICLE_MARKDOWN_MARKER", Message: "正文仍有未渲染的 Markdown 加粗标记，请检查中文紧邻强调写法"})
	}
	return issues
}

func formatReportPeriod(source SourceDocument) (string, string) {
	if source.PeriodStart.IsZero() {
		return "", ""
	}
	startCN := fmt.Sprintf("%d 年 %d 月 %d 日", source.PeriodStart.Year(), source.PeriodStart.Month(), source.PeriodStart.Day())
	startDotted := source.PeriodStart.Format("2006.01.02")
	if source.ReportType != domain.ReportWeekly || source.PeriodEnd.IsZero() {
		return startCN, startDotted
	}
	endCN := fmt.Sprintf("%d 年 %d 月 %d 日", source.PeriodEnd.Year(), source.PeriodEnd.Month(), source.PeriodEnd.Day())
	return startCN + "—" + endCN, startDotted + "–" + source.PeriodEnd.Format("2006.01.02")
}

func lintArticlePeriod(periodNode *nethtml.Node, source SourceDocument, dateCN string) []domain.Issue {
	if source.ReportType == domain.ReportWeekly {
		if periodNode.Data != "p" && periodNode.Data != "blockquote" {
			return []domain.Issue{{Code: "ARTICLE_DATE_RANGE", Message: "周报报头后的日期范围必须是段落或引用块"}}
		}
		periodText := firstPeriodParagraphText(periodNode)
		start, end, issue := extractDateRange(periodText)
		if issue != nil || !start.Equal(source.PeriodStart) || !end.Equal(source.PeriodEnd) {
			return []domain.Issue{{Code: "ARTICLE_DATE_RANGE", Message: fmt.Sprintf("周报报头后的日期范围必须表示“%s”", dateCN)}}
		}
		return nil
	}
	if periodNode.Data != "p" || !regexp.MustCompile(`^生成(?:日期|时间)\s*[：:]\s*`+regexp.QuoteMeta(dateCN)).MatchString(nodeText(periodNode)) {
		return []domain.Issue{{Code: "ARTICLE_DATE", Message: fmt.Sprintf("日报报头后的日期或时间必须以“%s”开头", dateCN)}}
	}
	return nil
}

func firstPeriodParagraphText(node *nethtml.Node) string {
	if node.Data == "p" {
		return nodeText(node)
	}
	for child := node.FirstChild; child != nil; child = child.NextSibling {
		if child.Type == nethtml.ElementNode && child.Data == "p" {
			return nodeText(child)
		}
	}
	return ""
}

func reportTypeName(reportType domain.ReportType) string {
	if reportType == domain.ReportWeekly {
		return "周报"
	}
	return "日报"
}

func lintStylesAndScripts(root *nethtml.Node) []domain.Issue {
	issues := make([]domain.Issue, 0)
	styles := elements(root, "style")
	if len(styles) != 1 {
		issues = append(issues, domain.Issue{Code: "STYLE_COUNT", Message: "页面必须且只能包含一个内联 style"})
	} else {
		issues = append(issues, lintCSS(nodeText(styles[0]))...)
	}
	scripts := elements(root, "script")
	if len(scripts) != 2 {
		issues = append(issues, domain.Issue{Code: "SCRIPT_COUNT", Message: "页面必须包含模板预置的两个内联脚本"})
	}
	allScripts := strings.Builder{}
	for index, scriptNode := range scripts {
		scriptText := nodeText(scriptNode)
		allScripts.WriteString(scriptText)
		allScripts.WriteByte('\n')
		if attribute(scriptNode, "src") != "" {
			issues = append(issues, domain.Issue{Code: "SCRIPT_EXTERNAL", Message: "单页面不允许引用外部 JavaScript"})
		}
		if _, err := parser.ParseFile(nil, fmt.Sprintf("inline-%d.js", index+1), scriptText, 0); err != nil {
			issues = append(issues, domain.Issue{Code: "JS_SYNTAX", Message: fmt.Sprintf("第 %d 个脚本语法错误：%v", index+1, err)})
		}
	}
	for _, marker := range []string{
		"safely('标题智能换行'",
		"new Intl.Segmenter('zh-CN'",
		"strongBreakPunctuation",
		"boundaryPenalty",
		"noBreakGroups",
		"lineLimit",
		"heading-break",
	} {
		if !strings.Contains(allScripts.String(), marker) {
			issues = append(issues, domain.Issue{Code: "JS_HEADING_WRAP", Message: fmt.Sprintf("标题分词脚本缺少必要标记：%s", marker)})
		}
	}
	return issues
}

func lintCSS(styleText string) []domain.Issue {
	issues := make([]domain.Issue, 0)
	css := cssparser.NewParser(parse.NewInputString(styleText), false)
	for {
		grammar, _, _ := css.Next()
		if grammar != cssparser.ErrorGrammar {
			continue
		}
		if err := css.Err(); err != nil && err != io.EOF {
			issues = append(issues, domain.Issue{Code: "CSS_SYNTAX", Message: fmt.Sprintf("CSS 语法错误：%v", err)})
		}
		break
	}
	if strings.Contains(styleText, "text-wrap: balance") {
		issues = append(issues, domain.Issue{Code: "CSS_HEADING_BALANCE", Message: "标题不得使用会机械均衡行宽的 text-wrap: balance"})
	}
	match := headingRulePattern.FindStringSubmatch(styleText)
	if len(match) != 2 {
		issues = append(issues, domain.Issue{Code: "CSS_HEADING_RULE", Message: "缺少标题自然换行规则"})
		return issues
	}
	required := []string{"text-wrap: pretty", "word-break: normal", "line-break: strict", "overflow-wrap: break-word"}
	for _, declaration := range required {
		if !strings.Contains(match[1], declaration) {
			issues = append(issues, domain.Issue{Code: "CSS_HEADING_RULE", Message: fmt.Sprintf("标题规则必须包含 %s", declaration)})
		}
	}
	headingWordMatch := headingWordRulePattern.FindStringSubmatch(styleText)
	if len(headingWordMatch) != 2 || !strings.Contains(headingWordMatch[1], "white-space: nowrap") {
		issues = append(issues, domain.Issue{Code: "CSS_HEADING_WORD", Message: "标题词组规则必须包含 white-space: nowrap"})
	}
	headingBreakMatch := headingBreakRulePattern.FindStringSubmatch(styleText)
	if len(headingBreakMatch) != 2 || !strings.Contains(headingBreakMatch[1], "display: block") || !strings.Contains(headingBreakMatch[1], "height: 0") {
		issues = append(issues, domain.Issue{Code: "CSS_HEADING_BREAK", Message: "标题动态断点规则必须包含 display: block 和 height: 0"})
	}
	return issues
}

func expectMeta(issues *[]domain.Issue, values map[string]string, name, expected, code string) {
	if values[name] != expected {
		*issues = append(*issues, domain.Issue{Code: code, Message: fmt.Sprintf("meta %s 必须是“%s”", name, expected)})
	}
}

func hashBytes(value []byte) string {
	digest := sha256.Sum256(value)
	return hex.EncodeToString(digest[:])
}

func hashTemplate(templateText string) string {
	normalized := strings.ReplaceAll(strings.ReplaceAll(templateText, "\r\n", "\n"), "\r", "\n")
	return hashBytes([]byte(normalized))
}

// TemplateHash 返回当前编译模板的稳定 SHA-256。
func TemplateHash(templateText string) string {
	return hashTemplate(templateText)
}

func firstDifference(actual, expected string) string {
	limit := min(len(actual), len(expected))
	offset := 0
	for offset < limit && actual[offset] == expected[offset] {
		offset++
	}
	line := strings.Count(actual[:offset], "\n") + 1
	lastBreak := strings.LastIndex(actual[:offset], "\n")
	column := offset - lastBreak
	return fmt.Sprintf("第 %d 行第 %d 列附近", line, column)
}

func elements(root *nethtml.Node, name string) []*nethtml.Node {
	result := make([]*nethtml.Node, 0)
	walkNodes(root, func(node *nethtml.Node) {
		if node.Type == nethtml.ElementNode && node.Data == name {
			result = append(result, node)
		}
	})
	return result
}

func walkNodes(root *nethtml.Node, visit func(*nethtml.Node)) {
	visit(root)
	for child := root.FirstChild; child != nil; child = child.NextSibling {
		walkNodes(child, visit)
	}
}

func nodeText(root *nethtml.Node) string {
	var builder strings.Builder
	walkNodes(root, func(node *nethtml.Node) {
		if node.Type == nethtml.TextNode {
			builder.WriteString(node.Data)
		}
	})
	return strings.TrimSpace(builder.String())
}

func textWithoutCode(root *nethtml.Node) string {
	var builder strings.Builder
	var visit func(*nethtml.Node)
	visit = func(node *nethtml.Node) {
		if node.Type == nethtml.ElementNode && (node.Data == "code" || node.Data == "pre") {
			return
		}
		if node.Type == nethtml.TextNode {
			builder.WriteString(node.Data)
		}
		for child := node.FirstChild; child != nil; child = child.NextSibling {
			visit(child)
		}
	}
	visit(root)
	return builder.String()
}

func elementChildren(root *nethtml.Node) []*nethtml.Node {
	result := make([]*nethtml.Node, 0)
	for child := root.FirstChild; child != nil; child = child.NextSibling {
		if child.Type == nethtml.ElementNode {
			result = append(result, child)
		}
	}
	return result
}

func attribute(node *nethtml.Node, name string) string {
	for _, attr := range node.Attr {
		if attr.Key == name {
			return attr.Val
		}
	}
	return ""
}

func hasClass(node *nethtml.Node, class string) bool {
	for _, value := range strings.Fields(attribute(node, "class")) {
		if value == class {
			return true
		}
	}
	return false
}

func headingTexts(root *nethtml.Node, name string) []string {
	result := make([]string, 0)
	for _, node := range elements(root, name) {
		result = append(result, nodeText(node))
	}
	return result
}

func equalStrings(left, right []string) bool {
	if len(left) != len(right) {
		return false
	}
	for index := range left {
		if left[index] != right[index] {
			return false
		}
	}
	return true
}
