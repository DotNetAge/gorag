package utils

import (
	"crypto/sha256"
	"encoding/hex"
)

// GenerateID 生成永不重复、同一个文件永远相同的 DocID
func GenerateID(content []byte) string {
	hasher := sha256.New()
	hasher.Write(content)
	hash := hasher.Sum(nil)
	return hex.EncodeToString(hash) // 64位字符串
}
