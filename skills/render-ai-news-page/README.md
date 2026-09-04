# render-ai-news-page

将用户通过文件、当前消息或 Agent 上下文独立提供的日报或周报新闻文本渲染为固定模板单页面 HTML。日报与周报由显式参数选择并执行各自独立的 lint 规则。普通生成与开发维护通过 [SKILL.md](SKILL.md) 路由；工具构建和源码说明位于 [go-cli/README.md](go-cli/README.md)。

本 skill 不负责新闻搜集，不对正文做摘要或压缩。
