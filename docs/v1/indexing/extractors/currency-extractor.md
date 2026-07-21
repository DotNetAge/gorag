# CurrencyExtractor

`CurrencyExtractor` 是 GoRAG 中用于提取货币金额的内置提取器。

## 功能

- 提取人民币金额（如：¥100、100元、100人民币）
- 提取美元金额（如：$100、100USD、100美元）
- 提取欧元金额（如：€100、100EUR、100欧元）
- 提取英镑金额（如：£100、100GBP、100英镑）
- 提取日元金额（如：¥100、100JPY、100日元）

## 实现原理

基于 `PatternExtractor`，使用：

1. **货币符号模式**：识别各种货币符号
2. **货币代码模式**：识别 ISO 货币代码
3. **金额模式**：识别数字金额
4. **货币单位模式**：识别货币单位（元、美元等）

## 提取规则

| 规则 | 示例 | 说明 |
|------|------|------|
| 人民币 | ¥100、100元、100人民币 | 人民币金额 |
| 美元 | $100、100USD、100美元 | 美元金额 |
| 欧元 | €100、100EUR、100欧元 | 欧元金额 |
| 英镑 | £100、100GBP、100英镑 | 英镑金额 |
| 日元 | ¥100、100JPY、100日元 | 日元金额 |

## 配置选项

| 参数 | 类型 | 说明 | 默认值 |
|------|------|------|--------|
| `Currencies` | []string | 支持的货币类型 | ["CNY", "USD", "EUR", "GBP", "JPY"] |
| `MinValue` | float64 | 最小金额 | 0 |
| `MaxValue` | float64 | 最大金额 | 1e12 |

## 使用示例

```go
// 基本使用
currencyExtractor := extractors.CurrencyExtractor

// 提取文本中的货币金额
text := "商品价格为¥999元，美元价格为$149。"
entities, edges := currencyExtractor.Extract(text)

// 结果：
// entities = [
//   {Type: "Currency", Value: "¥999元"},
//   {Type: "Currency", Value: "$149"}
// ]
```

## 扩展方法

```go
// 添加自定义货币模式
currencyExtractor.AddPattern("Currency", `¥(\d+(\.\d+)?)元?`) // 人民币格式

// 添加自定义货币单位
currencyExtractor.AddPattern("Currency", `(\d+(\.\d+)?)\s*(美元|USD)`) // 美元格式

// 添加其他货币
currencyExtractor.AddPattern("Currency", `(\d+(\.\d+)?)\s*(澳元|AUD)`) // 澳元
```