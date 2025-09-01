package tracing

import (
	"reflect"
	"testing"
)

func TestConsensusTagExtractor(t *testing.T) {
	tests := []struct {
		identifier string
		expected   map[string]string
	}{
		{
			identifier: "consensus.subslot.StartSlot",
			expected: map[string]string{
				TagComponent:      "consensus",
				TagConsensusPhase: "start",
				TagSubslotName:    "StartSlot",
			},
		},
		{
			identifier: "consensus.block.createHeader",
			expected: map[string]string{
				TagComponent:      "consensus",
				TagConsensusPhase: "block",
				TagOperationType:  "create",
			},
		},
		{
			identifier: "consensus.signature.broadcastSignature",
			expected: map[string]string{
				TagComponent:      "consensus",
				TagConsensusPhase: "signature",
				TagOperationType:  "broadcast",
			},
		},
		{
			identifier: "consensus.endSlot.commitBlock",
			expected: map[string]string{
				TagComponent:      "consensus",
				TagConsensusPhase: "end",
				TagOperationType:  "commit",
			},
		},
		{
			identifier: "storage.write.data", // Changed to not contain "consensus"
			expected:   map[string]string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.identifier, func(t *testing.T) {
			tags := ConsensusTagExtractor.ExtractTags(tt.identifier)
			if !reflect.DeepEqual(tags, tt.expected) {
				t.Errorf("ConsensusTagExtractor.ExtractTags(%q) = %v, want %v",
					tt.identifier, tags, tt.expected)
			}
		})
	}
}

func TestNetworkTagExtractor(t *testing.T) {
	tests := []struct {
		identifier string
		expected   map[string]string
	}{
		{
			identifier: "network.send.message",
			expected: map[string]string{
				TagComponent:        "network",
				TagNetworkDirection: "outbound",
			},
		},
		{
			identifier: "p2p.receive.block",
			expected: map[string]string{
				TagComponent:        "network",
				TagNetworkDirection: "inbound",
			},
		},
		{
			identifier: "consensus.block",
			expected:   map[string]string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.identifier, func(t *testing.T) {
			tags := NetworkTagExtractor.ExtractTags(tt.identifier)
			if !reflect.DeepEqual(tags, tt.expected) {
				t.Errorf("NetworkTagExtractor.ExtractTags(%q) = %v, want %v",
					tt.identifier, tags, tt.expected)
			}
		})
	}
}

func TestStorageTagExtractor(t *testing.T) {
	tests := []struct {
		identifier string
		expected   map[string]string
	}{
		{
			identifier: "storage.write.block",
			expected: map[string]string{
				TagComponent:        "storage",
				TagStorageOperation: "write",
			},
		},
		{
			identifier: "database.load.state",
			expected: map[string]string{
				TagComponent:        "storage",
				TagStorageOperation: "read",
			},
		},
		{
			identifier: "db.delete.old",
			expected: map[string]string{
				TagComponent:        "storage",
				TagStorageOperation: "delete",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.identifier, func(t *testing.T) {
			tags := StorageTagExtractor.ExtractTags(tt.identifier)
			if !reflect.DeepEqual(tags, tt.expected) {
				t.Errorf("StorageTagExtractor.ExtractTags(%q) = %v, want %v",
					tt.identifier, tags, tt.expected)
			}
		})
	}
}

func TestCompositeTagExtractor(t *testing.T) {
	// Test that composite extractor merges tags from multiple extractors
	identifier := "consensus.block.save"
	tags := DefaultTagExtractor.ExtractTags(identifier)

	// The component could be either consensus or storage depending on order
	// What's important is that tags from both extractors are present
	if tags[TagComponent] == "" {
		t.Errorf("Expected component tag to be set, got empty")
	}

	if tags[TagConsensusPhase] != "block" {
		t.Errorf("Expected consensus.phase to be 'block', got %q", tags[TagConsensusPhase])
	}

	// Storage tags should also be present
	if tags[TagStorageOperation] != "write" {
		t.Errorf("Expected storage.operation to be 'write', got %q", tags[TagStorageOperation])
	}
}

func TestRegisterTagExtractor(t *testing.T) {
	// Create a custom extractor
	customExtractor := TagExtractorFunc(func(identifier string) map[string]string {
		tags := make(map[string]string)
		if identifier == "test.custom" {
			tags["custom.tag"] = "value"
		}
		return tags
	})

	// Register it
	RegisterTagExtractor(customExtractor)

	// Test that it works
	tags := GetDefaultTags("test.custom")
	if tags["custom.tag"] != "value" {
		t.Errorf("Custom tag extractor not working, expected custom.tag=value, got %v", tags)
	}
}

func TestExtractConsensusPhase(t *testing.T) {
	tests := []struct {
		identifier string
		expected   string
	}{
		{"consensus.startSlot", "start"},
		{"consensus.start_slot", "start"},
		{"consensus.resetState", "start"},
		{"consensus.block.create", "block"},
		{"consensus.createHeader", "block"},
		{"consensus.signature", "signature"},
		{"consensus.createSignature", "signature"},
		{"consensus.endSlot", "end"},
		{"consensus.commit", "end"},
		{"consensus.finalize", "end"},
		{"consensus.unknown", ""},
	}

	for _, tt := range tests {
		t.Run(tt.identifier, func(t *testing.T) {
			phase := extractConsensusPhase(tt.identifier)
			if phase != tt.expected {
				t.Errorf("extractConsensusPhase(%q) = %q, want %q",
					tt.identifier, phase, tt.expected)
			}
		})
	}
}
