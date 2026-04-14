package chunker

// Options contains common configuration for all chunkers
type Options struct {
	ChunkSize           int      // chunk size in characters
	Overlap             int      // overlap size in characters
	MinChunkSize        int      // minimum chunk size
	MaxChunkSize        int      // maximum chunk size
	MaxSentences        int      // max sentences per chunk for sentence chunker
	MaxParagraphs       int      // max paragraphs per chunk for paragraph chunker
	SimilarityThreshold float32  // similarity threshold for semantic chunker
	ParentSize          int      // parent chunk size for ParentDoc
	ChildSize           int      // child chunk size for ParentDoc
	Separators          []string // separator list for recursive chunker
}

// Option is a functional option for configuring chunkers
type Option func(*Options)

// DefaultOptions returns the default configuration
func DefaultOptions() Options {
	return Options{
		ChunkSize:           DefaultChunkSize,
		Overlap:             DefaultOverlap,
		MinChunkSize:        MinChunkSize,
		MaxChunkSize:        MaxChunkSize,
		MaxSentences:        DefaultMaxSentences,
		MaxParagraphs:       DefaultMaxParagraphs,
		SimilarityThreshold: DefaultSimilarityThreshold,
		ParentSize:          DefaultParentSize,
		ChildSize:           DefaultChildSize,
		Separators:          DefaultSeparators(),
	}
}

// WithChunkSize sets the chunk size
func WithChunkSize(size int) Option {
	return func(o *Options) {
		o.ChunkSize = size
	}
}

// WithOverlap sets the overlap size
func WithOverlap(overlap int) Option {
	return func(o *Options) {
		o.Overlap = overlap
	}
}

// WithMaxChunkSize sets the maximum chunk size
func WithMaxChunkSize(maxSize int) Option {
	return func(o *Options) {
		o.MaxChunkSize = maxSize
	}
}

// WithMinChunkSize sets the minimum chunk size
func WithMinChunkSize(minSize int) Option {
	return func(o *Options) {
		o.MinChunkSize = minSize
	}
}

// WithMaxSentences sets the maximum number of sentences per chunk
func WithMaxSentences(maxSentences int) Option {
	return func(o *Options) {
		o.MaxSentences = maxSentences
	}
}

// WithMaxParagraphs sets the maximum number of paragraphs per chunk
func WithMaxParagraphs(maxParagraphs int) Option {
	return func(o *Options) {
		o.MaxParagraphs = maxParagraphs
	}
}

// WithSimilarityThreshold sets the similarity threshold for semantic chunking
func WithSimilarityThreshold(threshold float32) Option {
	return func(o *Options) {
		o.SimilarityThreshold = threshold
	}
}

// WithParentSize sets the parent chunk size
func WithParentSize(size int) Option {
	return func(o *Options) {
		o.ParentSize = size
	}
}

// WithChildSize sets the child chunk size
func WithChildSize(size int) Option {
	return func(o *Options) {
		o.ChildSize = size
	}
}

// WithSeparators sets the separator list
func WithSeparators(separators []string) Option {
	return func(o *Options) {
		o.Separators = separators
	}
}

// DefaultSeparators returns the default separator list (priority from high to low)
func DefaultSeparators() []string {
	return []string{
		"\n\n\n", // section boundary
		"\n\n",   // paragraph separator
		"\n",     // line break
		"。",     // Chinese period
		".",     // English period
		"！",     // Chinese exclamation
		"!",     // English exclamation
		"？",     // Chinese question mark
		"?",     // English question mark
		"；",     // Chinese semicolon
		";",     // English semicolon
		"，",     // Chinese comma
		",",     // English comma
		" ",     // space
		"",      // character (last resort)
	}
}