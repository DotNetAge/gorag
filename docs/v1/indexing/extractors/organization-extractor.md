# OrganizationExtractor

`OrganizationExtractor` 是 GoRAG 中用于提取组织机构名称的内置提取器。

## 功能

- 提取公司名称（如：苹果公司、Microsoft）
- 提取政府机构（如：教育部、国务院）
- 提取学校和教育机构（如：北京大学、哈佛大学）
- 提取社会组织（如：红十字会、联合国）

## 实现原理

基于 `DictionaryExtractor`，使用：

1. **后缀词典**：包含常见组织后缀（公司、集团、大学、协会等）
2. **前缀词典**：包含常见组织前缀（中国、美国、国际等）
3. **模式匹配**：识别组织名称的常见组合模式

## 提取规则

| 规则 | 示例 | 说明 |
|------|------|------|
| 公司名称 | 苹果公司、Microsoft Corporation | 名称 + 公司后缀 |
| 政府机构 | 教育部、State Department | 部门名称 |
| 教育机构 | 北京大学、Harvard University | 名称 + 教育机构后缀 |
| 社会组织 | 红十字会、United Nations | 组织名称 |

## 配置选项

| 参数 | 类型 | 说明 | 默认值 |
|------|------|------|--------|
| `MinLength` | int | 最小组织名称长度 | 2 |
| `MaxLength` | int | 最大组织名称长度 | 20 |
| `IncludeAbbreviations` | bool | 是否包含缩写 | true |

## 使用示例

```go
// 基本使用
orgExtractor := extractors.OrganizationExtractor

// 提取文本中的组织机构
text := "苹果公司和微软公司都是科技巨头，北京大学是中国顶尖学府。"
entities, edges := orgExtractor.Extract(text)

// 结果：
// entities = [
//   {Type: "Organization", Value: "苹果公司"},
//   {Type: "Organization", Value: "微软公司"},
//   {Type: "Organization", Value: "北京大学"}
// ]
```

## 扩展方法

```go
// 添加自定义组织后缀
orgExtractor.AddDictionary("Organization", []string{"科技公司", "研究院", "实验室"})

// 添加自定义组织前缀
orgExtractor.AddDictionary("Organization", []string{"国家", "省级", "市级"})

// 添加自定义模式
orgExtractor.AddPattern("Organization", `(中国|美国|日本)?.+?(银行|保险|证券)`) // 金融机构
```