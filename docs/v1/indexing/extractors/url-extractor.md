# URLExtractor

`URLExtractor` 是 GoRAG 中用于提取 URL 链接的内置提取器。

## 功能

- 提取 HTTP URL（如：http://example.com）
- 提取 HTTPS URL（如：https://example.com）
- 提取带路径的 URL（如：https://example.com/path/to/page）
- 提取带查询参数的 URL（如：https://example.com/search?q=keyword）
- 提取带锚点的 URL（如：https://example.com/page#section）

## 实现原理

基于 `RegexExtractor`，使用：

1. **URL 正则**：符合 RFC 3986 标准的 URL 格式
2. **协议验证**：识别 HTTP 和 HTTPS 协议
3. **域名验证**：确保域名部分符合规范
4. **路径处理**：支持带路径、查询参数和锚点的 URL

## 提取规则

| 规则 | 示例 | 说明 |
|------|------|------|
| HTTP URL | http://example.com | HTTP 协议 URL |
| HTTPS URL | https://example.com | HTTPS 协议 URL |
| 带路径 | https://example.com/path | 带路径的 URL |
| 带查询参数 | https://example.com/search?q=keyword | 带查询参数的 URL |
| 带锚点 | https://example.com/page#section | 带锚点的 URL |

## 配置选项

| 参数 | 类型 | 说明 | 默认值 |
|------|------|------|--------|
| `IncludePath` | bool | 是否包含路径 | true |
| `IncludeQuery` | bool | 是否包含查询参数 | true |
| `IncludeFragment` | bool | 是否包含锚点 | true |
| `MaxLength` | int | 最大 URL 长度 | 2048 |

## 使用示例

```go
// 基本使用
urlExtractor := extractors.URLExtractor

// 提取文本中的 URL
text := "请访问 https://example.com 或 http://test.com/path?query=value"
entities, edges := urlExtractor.Extract(text)

// 结果：
// entities = [
//   {Type: "URL", Value: "https://example.com"},
//   {Type: "URL", Value: "http://test.com/path?query=value"}
// ]
```

## 扩展方法

```go
// 添加自定义 URL 模式
urlExtractor.AddPattern("URL", `https?://[\w\-]+(\.[\w\-]+)+([\w\-\.,@?^=%&:/~\+#]*[\w\-\@?^=%&/~\+#])?`) // 标准 URL 正则

// 添加特定域名的 URL
urlExtractor.AddPattern("URL", `https?://(www\.)?google\.com[\w\-\.,@?^=%&:/~\+#]*`) // Google 域名
```