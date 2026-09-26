package ast

// ExpressionStatement evaluates an expression for its side effects.
type ExpressionStatement struct {
	Base
	Expression Expression `json:"expression"`
}

func (*ExpressionStatement) node()      {}
func (*ExpressionStatement) statement() {}

// VariableDeclaration declares var or const.
type VariableDeclaration struct {
	Base
	Name     string          `json:"name"`
	Type     string          `json:"type,omitempty"`
	Value    Expression      `json:"value,omitempty"`
	Constant bool            `json:"constant,omitempty"`
	Inferred bool            `json:"inferred,omitempty"`
	Static   bool            `json:"static,omitempty"`
	Getter   []Statement     `json:"getter,omitempty"`
	Setter   *PropertySetter `json:"setter,omitempty"`
}

func (*VariableDeclaration) node()      {}
func (*VariableDeclaration) statement() {}

// Assignment assigns Value to Target using Operator.
type Assignment struct {
	Base
	Target   Expression `json:"target"`
	Operator string     `json:"operator"`
	Value    Expression `json:"value"`
}

func (*Assignment) node()      {}
func (*Assignment) statement() {}

// ReturnStatement optionally returns a value.
type ReturnStatement struct {
	Base
	Value Expression `json:"value,omitempty"`
}

func (*ReturnStatement) node()      {}
func (*ReturnStatement) statement() {}

// Branch is one if/elif condition and body.
type Branch struct {
	Condition Expression  `json:"condition"`
	Body      []Statement `json:"body"`
}

// IfStatement contains its if/elif branches and optional else body.
type IfStatement struct {
	Base
	Branches []Branch    `json:"branches"`
	Else     []Statement `json:"else,omitempty"`
}

func (*IfStatement) node()      {}
func (*IfStatement) statement() {}

// WhileStatement is a condition-controlled loop.
type WhileStatement struct {
	Base
	Condition Expression  `json:"condition"`
	Body      []Statement `json:"body"`
}

func (*WhileStatement) node()      {}
func (*WhileStatement) statement() {}

// ForStatement iterates one variable over an expression.
type ForStatement struct {
	Base
	Variable string      `json:"variable"`
	Type     string      `json:"type,omitempty"`
	Iterable Expression  `json:"iterable"`
	Body     []Statement `json:"body"`
}

func (*ForStatement) node()      {}
func (*ForStatement) statement() {}

// KeywordStatement is pass, break, or continue.
type KeywordStatement struct {
	Base
	Keyword string `json:"keyword"`
}

func (*KeywordStatement) node()      {}
func (*KeywordStatement) statement() {}

// Parameter is a function or signal parameter.
type Parameter struct {
	Name    string     `json:"name"`
	Type    string     `json:"type,omitempty"`
	Default Expression `json:"default,omitempty"`
}

// FunctionDeclaration declares a function.
type FunctionDeclaration struct {
	Base
	Name       string      `json:"name"`
	Parameters []Parameter `json:"parameters"`
	ReturnType string      `json:"return_type,omitempty"`
	Static     bool        `json:"static,omitempty"`
	Abstract   bool        `json:"abstract,omitempty"`
	Body       []Statement `json:"body"`
}

func (*FunctionDeclaration) node()      {}
func (*FunctionDeclaration) statement() {}

// ClassDeclaration declares an inner class.
type ClassDeclaration struct {
	Base
	Name    string      `json:"name"`
	Extends string      `json:"extends,omitempty"`
	Body    []Statement `json:"body"`
}

func (*ClassDeclaration) node()      {}
func (*ClassDeclaration) statement() {}

// SignalDeclaration declares a signal.
type SignalDeclaration struct {
	Base
	Name       string      `json:"name"`
	Parameters []Parameter `json:"parameters,omitempty"`
}

func (*SignalDeclaration) node()      {}
func (*SignalDeclaration) statement() {}

// EnumMember is one enum value.
type EnumMember struct {
	Name     string     `json:"name"`
	Value    Expression `json:"value,omitempty"`
	Comments []*Comment `json:"comments,omitempty"`
}

// PropertySetter stores a property's setter parameter and body.
type PropertySetter struct {
	Parameter string      `json:"parameter"`
	Body      []Statement `json:"body"`
}

// EnumDeclaration declares a named or anonymous enum.
type EnumDeclaration struct {
	Base
	Name    string       `json:"name,omitempty"`
	Members []EnumMember `json:"members"`
}

func (*EnumDeclaration) node()      {}
func (*EnumDeclaration) statement() {}

// MatchCase contains comma-separated patterns and a body.
type MatchCase struct {
	Patterns []Expression `json:"patterns"`
	Guard    Expression   `json:"guard,omitempty"`
	Body     []Statement  `json:"body"`
}

// MatchStatement performs pattern matching.
type MatchStatement struct {
	Base
	Value Expression  `json:"value"`
	Cases []MatchCase `json:"cases"`
}

func (*MatchStatement) node()      {}
func (*MatchStatement) statement() {}
