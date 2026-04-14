# Extractor 体系

Extractor 是 GoRAG 中负责实体提取的核心组件，提供从文本中识别和提取各种实体的能力。

## 设计理念

- **规则驱动**：基于词典、正则表达式和模式匹配，确保高性能和可预测性
- **模块化**：每种实体类型对应专门的提取器
- **可组合**：通过 CompositeExtractor 组合多个提取器
- **可扩展**：支持开发者自定义提取器

## 提取器类型

| 类型 | 特点 | 适用场景 |
|------|------|----------|
| **DictionaryExtractor** | 基于词典和模式 | 人名、组织机构、地点等 |
| **RegexExtractor** | 基于正则表达式 | 邮箱、电话、URL、IP 等 |
| **PatternExtractor** | 基于复杂模式 | 时间、数值、货币等 |
| **CompositeExtractor** | 组合多个提取器 | 综合场景 |

## 内置提取器

GoRAG 提供 10+ 种内置提取器，覆盖常见实体类型：

- [PersonExtractor](person-extractor.md) - 提取人名
- [OrganizationExtractor](organization-extractor.md) - 提取组织机构
- [LocationExtractor](location-extractor.md) - 提取地点
- [TimeExtractor](time-extractor.md) - 提取时间
- [NumberExtractor](number-extractor.md) - 提取数值
- [EmailExtractor](email-extractor.md) - 提取邮箱
- [PhoneExtractor](phone-extractor.md) - 提取电话
- [URLExtractor](url-extractor.md) - 提取 URL
- [CurrencyExtractor](currency-extractor.md) - 提取货币
- [IPExtractor](ip-extractor.md) - 提取 IP 地址

## 自定义提取器

开发者可以通过实现 `EntityExtractor` 接口创建自定义提取器：

```go
type EntityExtractor interface {
    Extract(text string) ([]Entity, []Edge)
    ExtractStream(ctx context.Context, textChan chan string) chan EntityEdgePair
    GetSupportedTypes() []string
}
```

## 使用示例

### 基本使用

```go
// 使用内置提取器
parser := parsing.NewParser()
parser.SetExtractor(extractors.PersonExtractor)

// 解析文档
doc, err := parser.Parse(ctx, content, "text/plain")
```

### 组合使用

```go
// 创建组合提取器
composite := extractors.NewCompositeExtractor()
composite.AddExtractor(extractors.PersonExtractor)
composite.AddExtractor(extractors.OrganizationExtractor)
composite.AddExtractor(extractors.LocationExtractor)

// 设置到解析器
parser.SetExtractor(composite)
```

### 自定义提取器

```go
// 实现自定义提取器
type ProductExtractor struct {
    extractors.RegexExtractor
}

func NewProductExtractor() *ProductExtractor {
    extractor := &ProductExtractor{
        RegexExtractor: *extractors.NewRegexExtractor("product"),
    }
    extractor.AddPattern("Product", `[A-Z]{2,4}-\d{3,5}`) // 产品型号
    return extractor
}

// 使用自定义提取器
productExtractor := NewProductExtractor()
composite.AddExtractor(productExtractor)
```