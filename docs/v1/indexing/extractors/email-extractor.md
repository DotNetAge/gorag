# EmailExtractor

`EmailExtractor` 是 GoRAG 中用于提取邮箱地址的内置提取器。

## 功能

- 提取标准邮箱地址（如：user@example.com）
- 提取企业邮箱（如：name@company.com）
- 提取教育邮箱（如：student@university.edu）
- 提取国际化邮箱（如：用户@域名.中国）

## 实现原理

基于 `RegexExtractor`，使用：

1. **邮箱正则**：符合 RFC 5322 标准的邮箱格式
2. **域名验证**：识别常见域名后缀
3. **本地部分验证**：确保本地部分符合规范

## 提取规则

| 规则       | 示例                   | 说明                 |
| ---------- | ---------------------- | -------------------- |
| 标准邮箱   | user@example.com       | 标准邮箱格式         |
| 企业邮箱   | name@company.com       | 企业域名邮箱         |
| 教育邮箱   | student@university.edu | 教育机构邮箱         |
| 国际化邮箱 | 用户@域名.中国         | 支持中文的国际化邮箱 |

## 配置选项

| 参数                   | 类型 | 说明               | 默认值 |
| ---------------------- | ---- | ------------------ | ------ |
| `IncludeInternational` | bool | 是否包含国际化邮箱 | true   |
| `MaxLength`            | int  | 最大邮箱长度       | 254    |

## 使用示例

```go
// 基本使用
emailExtractor := extractors.EmailExtractor

// 提取文本中的邮箱地址
text := "请联系我：john@example.com 或 support@company.com"
entities, edges := emailExtractor.Extract(text)

// 结果：
// entities = [
//   {Type: "Email", Value: "john@example.com"},
//   {Type: "Email", Value: "support@company.com"}
// ]
```

## 扩展方法

```go
// 添加自定义邮箱模式
emailExtractor.AddPattern("Email", `[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}`) // 标准邮箱正则

// 添加特定域名的邮箱
emailExtractor.AddPattern("Email", `[a-zA-Z0-9._%+-]+@(gmail|yahoo|outlook)\.com`) // 常见邮箱服务商
```