package core

// Loader 加载器接口，统一读取各类文件，输出原始二进制文档
type Loader interface {
	// Load 读取指定路径/URL的文件，返回原始文档
	Load(path string) (Document, error)
	// SupportTypes 返回支持的文件类型列表
	SupportTypes() []string
}
