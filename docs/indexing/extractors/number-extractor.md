# NumberExtractor

`NumberExtractor` 是 GoRAG 中用于提取数值的内置提取器。

## 功能

- 提取整数（如：123、4567）
- 提取小数（如：3.14、2.5）
- 提取百分比（如：50%、75.5%）
- 提取分数（如：1/2、3/4）
- 提取大数（如：100万、2.5亿）
- 提取序数（如：第一、第二、1st、2nd）

## 实现原理

基于 `PatternExtractor`，使用：

1. **数值正则**：识别各种数值格式
2. **单位模式**：识别数量单位（万、亿、千等）
3. **百分比模式**：识别百分比格式
4. **分数模式**：识别分数格式

## 提取规则

| 规则   | 示例                 | 说明         |
| ------ | -------------------- | ------------ |
| 整数   | 123、4567            | 整数数值     |
| 小数   | 3.14、2.5            | 小数数值     |
| 百分比 | 50%、75.5%           | 百分比格式   |
| 分数   | 1/2、3/4             | 分数格式     |
| 大数   | 100万、2.5亿         | 带单位的大数 |
| 序数   | 第一、第二、1st、2nd | 序数词       |

## 配置选项

| 参数              | 类型    | 说明         | 默认值 |
| ----------------- | ------- | ------------ | ------ |
| `MinValue`        | float64 | 最小数值     | 0      |
| `MaxValue`        | float64 | 最大数值     | 1e18   |
| `IncludeOrdinals` | bool    | 是否包含序数 | true   |
| `IncludeUnits`    | bool    | 是否包含单位 | true   |

## 使用示例

```go
// 基本使用
numberExtractor := extractors.NumberExtractor

// 提取文本中的数值
text := "公司今年营收达到1000万元，同比增长25.5%，位列行业第一。"
entities, edges := numberExtractor.Extract(text)

// 结果：
// entities = [
//   {Type: "Number", Value: "1000万元"},
//   {Type: "Number", Value: "25.5%"},
//   {Type: "Number", Value: "第一"}
// ]
```

## 扩展方法

```go
// 添加自定义数值模式
numberExtractor.AddPattern("Number", `(\d+)\s*(万|亿|千|百)`) // 带单位的数值

// 添加自定义序数
numberExtractor.AddPattern("Number", `(第[一二三四五六七八九十]+)`) // 中文序数

// 添加自定义分数
numberExtractor.AddPattern("Number", `(\d+)/(\d+)`) // 分数格式
```