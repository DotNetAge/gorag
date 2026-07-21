# LocationExtractor

`LocationExtractor` 是 GoRAG 中用于提取地点的内置提取器。

## 功能

- 提取国家名称（如：中国、美国、日本）
- 提取省份/州（如：北京市、加利福尼亚州）
- 提取城市（如：北京、上海、纽约）
- 提取地标（如：长城、天安门、埃菲尔铁塔）
- 提取行政区划（如：海淀区、朝阳区）

## 实现原理

基于 `DictionaryExtractor`，使用：

1. **行政区划词典**：包含国家、省份、城市、区县等层级信息
2. **地标词典**：包含著名地标和景点
3. **模式匹配**：识别地点的常见组合模式

## 提取规则

| 规则 | 示例 | 说明 |
|------|------|------|
| 国家 | 中国、美国、日本 | 国家名称 |
| 省份/州 | 北京市、加利福尼亚州 | 省级行政区划 |
| 城市 | 北京、上海、纽约 | 城市名称 |
| 地标 | 长城、天安门、埃菲尔铁塔 | 著名地标 |
| 行政区划 | 海淀区、朝阳区 | 区县级别行政区划 |

## 配置选项

| 参数 | 类型 | 说明 | 默认值 |
|------|------|------|--------|
| `MinLength` | int | 最小地点名称长度 | 2 |
| `MaxLength` | int | 最大地点名称长度 | 15 |
| `IncludeLandmarks` | bool | 是否包含地标 | true |

## 使用示例

```go
// 基本使用
locationExtractor := extractors.LocationExtractor

// 提取文本中的地点
text := "我去过中国北京的长城，还去过美国纽约的自由女神像。"
entities, edges := locationExtractor.Extract(text)

// 结果：
// entities = [
//   {Type: "Location", Value: "中国"},
//   {Type: "Location", Value: "北京"},
//   {Type: "Location", Value: "长城"},
//   {Type: "Location", Value: "美国"},
//   {Type: "Location", Value: "纽约"},
//   {Type: "Location", Value: "自由女神像"}
// ]
```

## 扩展方法

```go
// 添加自定义地点
locationExtractor.AddDictionary("Location", []string{"雄安新区", "粤港澳大湾区"})

// 添加自定义地标
locationExtractor.AddDictionary("Location", []string{"鸟巢", "水立方", "环球影城"})

// 添加自定义模式
locationExtractor.AddPattern("Location", `(\w+省|\w+市|\w+区|\w+县)`) // 行政区划模式
```