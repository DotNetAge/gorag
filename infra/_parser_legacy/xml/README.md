# 📄 XML Parser - XML 解析器

**状态：** ✅ 已完成  
**日期：** 2024-03-19  
**架构：** SAX 流式解析  

---

## 🌟 **核心特性**

### **1. SAX 流式解析**
```go
// ✅ 基于 encoding/xml.Decoder
// ✅ 逐 token 读取，内存效率 O(1)
// ✅ 支持 GB 级 XML 文件

parser := xml.NewParser()
chunks, err := parser.Parse(ctx, reader)
```

### **2. 智能内容清理**
```go
// ✅ 自动跳过注释（可配置）
// ✅ 忽略空白字符
// ✅ 保留文本内容

parser.SetPreserveComments(true)  // 保留注释
```

---

## 🚀 **快速开始**

```go
package main

import (
    "context"
    "os"
    "github.com/DotNetAge/gorag/parser/xml"
)

func main() {
    parser := xml.NewParser()
    
    file, _ := os.Open("data.xml")
    defer file.Close()
    
    ctx := context.Background()
    chunks, _ := parser.Parse(ctx, file)
    
    for _, chunk := range chunks {
        println(chunk.Content)
    }
}
```

---

## 📊 **测试结果**

```bash
$ go test -v -cover ./...
=== RUN   TestParser_Parse
--- PASS: TestParser_Parse (0.00s)
=== RUN   TestParser_ParseWithCallback
--- PASS: TestParser_ParseWithCallback (0.00s)
=== RUN   TestParser_EmptyXML
--- PASS: TestParser_EmptyXML (0.00s)
=== RUN   TestParser_LargeXML
--- PASS: TestParser_LargeXML (0.00s)
PASS
coverage: 91.1% of statements
```

---

*📍 位置：`/Users/ray/workspaces/gorag/gorag/parser/xml/`*  
*✅ 状态：完成并可用*  
*📅 完成日期：2024-03-19*
