package tracing

import (
	"maps"
	"strings"
	"sync"
)

// Tag key constants to avoid duplication
const (
	TagComponent        = "component"
	TagOperationType    = "operation.type"
	TagConsensusPhase   = "consensus.phase"
	TagSubslotName      = "subslot.name"
	TagNetworkDirection = "network.direction"
	TagStorageOperation = "storage.operation"
	TagVMType           = "vm.type"
	TagVMOperation      = "vm.operation"
)

// TagExtractor defines an interface for extracting tags from span identifiers
type TagExtractor interface {
	ExtractTags(identifier string) map[string]string
}

// TagExtractorFunc is a function adapter for TagExtractor
type TagExtractorFunc func(identifier string) map[string]string

func (f TagExtractorFunc) ExtractTags(identifier string) map[string]string {
	return f(identifier)
}

// CompositeTagExtractor combines multiple tag extractors
type CompositeTagExtractor struct {
	mu         sync.RWMutex
	extractors []TagExtractor
}

// NewCompositeTagExtractor creates a new composite tag extractor
func NewCompositeTagExtractor(extractors ...TagExtractor) *CompositeTagExtractor {
	return &CompositeTagExtractor{
		extractors: extractors,
	}
}

// ExtractTags runs all extractors and merges their results
func (c *CompositeTagExtractor) ExtractTags(identifier string) map[string]string {
	tags := make(map[string]string)
	c.mu.RLock()
	extractors := append([]TagExtractor(nil), c.extractors...)
	c.mu.RUnlock()
	for _, extractor := range extractors {
		maps.Copy(tags, extractor.ExtractTags(identifier))
	}
	return tags
}

// AddExtractor adds a new extractor to the composite
func (c *CompositeTagExtractor) AddExtractor(extractor TagExtractor) {
	c.mu.Lock()
	c.extractors = append(c.extractors, extractor)
	c.mu.Unlock()
}

// Default tag extractors

// ConsensusTagExtractor extracts tags for consensus operations
var ConsensusTagExtractor = TagExtractorFunc(func(identifier string) map[string]string {
	tags := make(map[string]string)

	// Only process if it contains "consensus" in the identifier
	if !strings.Contains(strings.ToLower(identifier), "consensus") {
		return tags
	}

	tags[TagComponent] = "consensus"

	// Extract phase
	phase := extractConsensusPhase(identifier)
	if phase != "" {
		tags[TagConsensusPhase] = phase
	}

	// Extract subslot name
	if strings.Contains(identifier, "subslot") {
		parts := strings.Split(identifier, ".")
		if len(parts) >= 3 {
			tags[TagSubslotName] = parts[2]
		}
	}

	// Extract specific operation details
	if strings.Contains(identifier, "create") {
		tags[TagOperationType] = "create"
	} else if strings.Contains(identifier, "broadcast") {
		tags[TagOperationType] = "broadcast"
	} else if strings.Contains(identifier, "commit") {
		tags[TagOperationType] = "commit"
	} else if strings.Contains(identifier, "aggregate") {
		tags[TagOperationType] = "aggregate"
	}

	return tags
})

func extractConsensusPhase(identifier string) string {
	// Check phases in order of priority (more specific first)
	phases := []struct {
		phase    string
		keywords []string
	}{
		{"start", []string{"startslot", "start_slot", "resetstate"}},
		{"end", []string{"endslot", "end_slot", "finalize"}},
		{"signature", []string{"signature", "createsignature", "broadcastsignature", "waitsignatures"}},
		{"block", []string{"block", "createheader", "createblock", "sendblock"}},
	}

	lowerID := strings.ToLower(identifier)
	lowerID = strings.ReplaceAll(lowerID, ".", "")
	lowerID = strings.ReplaceAll(lowerID, "_", "")
	lowerID = strings.ReplaceAll(lowerID, "-", "")

	for _, p := range phases {
		for _, keyword := range p.keywords {
			if strings.Contains(lowerID, keyword) {
				return p.phase
			}
		}
	}

	// Special case for commit which could be in end phase
	if strings.Contains(lowerID, "commit") {
		return "end"
	}

	return ""
}

// NetworkTagExtractor extracts tags for network operations
var NetworkTagExtractor = TagExtractorFunc(func(identifier string) map[string]string {
	tags := make(map[string]string)

	if !strings.Contains(identifier, "network") && !strings.Contains(identifier, "p2p") {
		return tags
	}

	tags[TagComponent] = "network"

	if strings.Contains(identifier, "send") || strings.Contains(identifier, "broadcast") {
		tags[TagNetworkDirection] = "outbound"
	} else if strings.Contains(identifier, "receive") || strings.Contains(identifier, "handle") {
		tags[TagNetworkDirection] = "inbound"
	}

	return tags
})

// StorageTagExtractor extracts tags for storage operations
var StorageTagExtractor = TagExtractorFunc(func(identifier string) map[string]string {
	tags := make(map[string]string)

	keywords := []string{"storage", "database", "db", "persist", "save", "load", "fetch"}
	found := false
	for _, keyword := range keywords {
		if strings.Contains(strings.ToLower(identifier), keyword) {
			found = true
			break
		}
	}

	if !found {
		return tags
	}

	tags[TagComponent] = "storage"

	if strings.Contains(identifier, "write") || strings.Contains(identifier, "save") || strings.Contains(identifier, "persist") {
		tags[TagStorageOperation] = "write"
	} else if strings.Contains(identifier, "read") || strings.Contains(identifier, "load") || strings.Contains(identifier, "fetch") {
		tags[TagStorageOperation] = "read"
	} else if strings.Contains(identifier, "delete") || strings.Contains(identifier, "remove") {
		tags[TagStorageOperation] = "delete"
	}

	return tags
})

// VMTagExtractor extracts tags for virtual machine operations
var VMTagExtractor = TagExtractorFunc(func(identifier string) map[string]string {
	tags := make(map[string]string)

	keywords := []string{"vm", "execute", "contract", "transaction", "process"}
	found := false
	for _, keyword := range keywords {
		if strings.Contains(strings.ToLower(identifier), keyword) {
			found = true
			break
		}
	}

	if !found {
		return tags
	}

	tags[TagComponent] = "vm"

	if strings.Contains(identifier, "contract") {
		tags[TagVMType] = "contract"
	} else if strings.Contains(identifier, "transaction") {
		tags[TagVMType] = "transaction"
	}

	if strings.Contains(identifier, "validate") {
		tags[TagVMOperation] = "validate"
	} else if strings.Contains(identifier, "execute") {
		tags[TagVMOperation] = "execute"
	}

	return tags
})

// DefaultTagExtractor is the default composite extractor with all standard extractors
var DefaultTagExtractor = NewCompositeTagExtractor(
	ConsensusTagExtractor,
	NetworkTagExtractor,
	StorageTagExtractor,
	VMTagExtractor,
)

// GetDefaultTags extracts tags using the default extractor
func GetDefaultTags(identifier string) map[string]string {
	return DefaultTagExtractor.ExtractTags(identifier)
}

// RegisterTagExtractor adds a custom tag extractor to the default extractor
func RegisterTagExtractor(extractor TagExtractor) {
	DefaultTagExtractor.AddExtractor(extractor)
}

// RegisterTagExtractorFunc adds a custom tag extractor function to the default extractor
func RegisterTagExtractorFunc(_ string, extractorFunc func(identifier string) map[string]string) {
	RegisterTagExtractor(TagExtractorFunc(extractorFunc))
}
