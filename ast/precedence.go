package ast

// Precedence levels of GDScript's operators, listed from the loosest binding to
// the tightest. They mirror the Precedence enum in Godot's
// modules/gdscript/gdscript_parser.h, so that an expression keeps its meaning
// through a parse and format round trip.
//
// Every binary operator in GDScript is left-associative, including "**", so the
// operand to the right of one is read one level above the operator itself.
const (
	// PrecedenceNone marks a token that is not an operator.
	PrecedenceNone = iota
	PrecedenceAssignment
	// PrecedenceCast is the level of "as", which binds looser than everything
	// else, so "a or b as int" casts the whole disjunction.
	PrecedenceCast
	PrecedenceTernary
	PrecedenceLogicalOr
	PrecedenceLogicalAnd
	PrecedenceLogicalNot
	// PrecedenceContentTest is the level of "in" and "not in", which bind
	// tighter than the logical operators but looser than a comparison.
	PrecedenceContentTest
	PrecedenceComparison
	PrecedenceBitwiseOr
	PrecedenceBitwiseXor
	PrecedenceBitwiseAnd
	PrecedenceBitwiseShift
	PrecedenceAddition
	PrecedenceFactor
	// PrecedenceSign is the level of a unary "+" or "-".
	PrecedenceSign
	// PrecedenceBitwiseNot is the level of a unary "~".
	PrecedenceBitwiseNot
	PrecedencePower
	// PrecedenceTypeTest is the level of "is" and "is not", which bind tighter
	// than every arithmetic operator, so "a ** b is int" tests b.
	PrecedenceTypeTest
	// PrecedenceAwait is the level of "await".
	PrecedenceAwait
	PrecedenceCall
	PrecedenceAttribute
	PrecedenceSubscript
	PrecedencePrimary
)

// OperatorPrecedence returns the level a binary operator binds at, or
// PrecedenceNone when operator is not a binary operator.
func OperatorPrecedence(operator string) int {
	switch operator {
	case "as":
		return PrecedenceCast
	case "or", "||":
		return PrecedenceLogicalOr
	case "and", "&&":
		return PrecedenceLogicalAnd
	case "in", "not in":
		return PrecedenceContentTest
	case "==", "!=", "<", "<=", ">", ">=":
		return PrecedenceComparison
	case "|":
		return PrecedenceBitwiseOr
	case "^":
		return PrecedenceBitwiseXor
	case "&":
		return PrecedenceBitwiseAnd
	case "<<", ">>":
		return PrecedenceBitwiseShift
	case "+", "-":
		return PrecedenceAddition
	case "*", "/", "%":
		return PrecedenceFactor
	case "**":
		return PrecedencePower
	case "is", "is not":
		return PrecedenceTypeTest
	default:
		return PrecedenceNone
	}
}

// UnaryOperandPrecedence returns the level a prefix operator reads its operand
// at, which is also the level the whole unary expression binds at. It is
// PrecedenceNone when operator is not a prefix operator.
func UnaryOperandPrecedence(operator string) int {
	switch operator {
	case "+", "-":
		return PrecedenceSign
	case "~":
		return PrecedenceBitwiseNot
	case "not", "!":
		return PrecedenceLogicalNot
	case "await":
		return PrecedenceAwait
	default:
		return PrecedenceNone
	}
}
