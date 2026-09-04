// Package assets 提供编译进 CLI 的唯一新闻页面模板。
package assets

import _ "embed"

// NewsPageTemplate 是普通生成唯一允许使用的页面外壳。
//
//go:embed news-page-template.html
var NewsPageTemplate string
