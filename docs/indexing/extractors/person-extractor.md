# PersonExtractor

`PersonExtractor` 是 GoRAG 中用于提取人名的内置提取器。

## 功能

- 提取中文人名（如：张三、李四）
- 提取英文人名（如：John Smith、Alice Johnson）
- 识别称谓（如：先生、女士、博士、教授）
- 支持复合人名（如：习近平主席、Dr. Martin Luther King）

## 实现原理

基于 `DictionaryExtractor`，使用：

1. **姓氏词典**：包含常见中文姓氏和英文姓氏
2. **称谓词典**：包含各种称谓（先生、女士、博士等）
3. **模式匹配**：识别姓名的常见组合模式

## 提取规则

| 规则 | 示例 | 说明 |
|------|------|------|
| 中文姓名 | 张三、李四 | 姓氏 + 名字 |
| 英文姓名 | John Smith、Alice Johnson | 名 + 姓 |
| 带称谓 | 张教授、Dr. Smith | 姓名 + 称谓 |
| 复合结构 | 习近平主席、President Obama | 姓名 + 职务 |

## 配置选项

| 参数 | 类型 | 说明 | 默认值 |
|------|------|------|--------|
| `MinLength` | int | 最小姓名长度 | 2 |
| `MaxLength` | int | 最大姓名长度 | 10 |
| `IncludeTitles` | bool | 是否包含称谓 | true |

## 使用示例

```go
// 基本使用
personExtractor := extractors.PersonExtractor

// 提取文本中的人名
text := "张三和李四一起去见王教授，途中遇到了Dr. Smith。"
entities, edges := personExtractor.Extract(text)

// 结果：
// entities = [
//   {Type: "Person", Value: "张三"},
//   {Type: "Person", Value: "李四"},
//   {Type: "Person", Value: "王教授"},
//   {Type: "Person", Value: "Dr. Smith"}
// ]
```

## 扩展方法

```go
// 添加自定义姓氏
personExtractor.AddDictionary("Person", []string{"诸葛", "司马", "欧阳"})

// 添加自定义称谓
personExtractor.AddDictionary("Person", []string{"院长", "主任", "工程师"})

// 添加自定义模式
personExtractor.AddPattern("Person", `([A-Z][a-z]+)\s+([A-Z][a-z]+)\s+(Jr|Sr|III)`) // 支持 Jr/Sr 后缀
```