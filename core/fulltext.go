package core

type FullTextStore interface {
	Add(doc *StructureNode) error
	Search(query string) ([]string, error)
}
