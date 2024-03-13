package scenjsonparse

import (
	scenexpressioninterpreter "github.com/klever-io/klever-go/kvm/scenarioexec/expression/interpreter"
	scenfileresolver "github.com/klever-io/klever-go/kvm/scenarioexec/fileresolver"
)

// Parser performs parsing of both json tests (older) and scenarios (new).
type Parser struct {
	ExprInterpreter                  scenexpressioninterpreter.ExprInterpreter
	AllowEsdtTxLegacySyntax          bool
	AllowEsdtLegacySetSyntax         bool
	AllowEsdtLegacyCheckSyntax       bool
	AllowSingleValueInCheckValueList bool
}

// NewParser provides a new Parser instance.
func NewParser(fileResolver scenfileresolver.FileResolver) Parser {
	return Parser{
		ExprInterpreter: scenexpressioninterpreter.ExprInterpreter{
			FileResolver: fileResolver,
		},
		AllowEsdtTxLegacySyntax:          true,
		AllowEsdtLegacySetSyntax:         true,
		AllowEsdtLegacyCheckSyntax:       true,
		AllowSingleValueInCheckValueList: true,
	}
}
