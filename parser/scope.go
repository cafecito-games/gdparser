package parser

import (
	"fmt"

	"github.com/cafecito-games/gdparser/ast"
	"github.com/cafecito-games/gdparser/token"
)

// The names a scope may hold, spelled as Godot spells them in
// SuiteNode::Local::get_name and in the member kinds parse_class_member reports.
const (
	parameterName  = "parameter"
	variableName   = "variable"
	constantName   = "constant"
	loopName       = "for loop iterator"
	bindName       = "pattern bind"
	functionName_  = "function"
	signalName     = "signal"
	classMemberKey = "class"
	enumName       = "enum"
)

// scope holds the names declared in one block, with the kind each was declared
// as, so that a redeclaration can say what the name already is.
type scope struct {
	kinds map[string]string
	// barrier marks a scope no lookup passes out of. Godot leaves a function
	// body's suite with no parent block, so a function cannot see the locals of
	// whatever encloses it, while a lambda body's suite does have one and so
	// cannot declare a local under a name from the function holding it.
	barrier bool
}

// pushScope opens a local scope. barrier says that names outside it are out of
// sight, which is what a function body is and a block is not.
func (p *parser) pushScope(barrier bool) {
	p.scopes = append(p.scopes, scope{kinds: map[string]string{}, barrier: barrier})
}

func (p *parser) popScope() {
	p.scopes = p.scopes[:len(p.scopes)-1]
}

// declareLocal records name in the innermost scope, or reports that the name is
// already taken by something the scope can see. Godot reads the same rule from
// SuiteNode::get_local, which walks the enclosing blocks.
func (p *parser) declareLocal(name token.Token, kind string) error {
	if len(p.scopes) == 0 {
		return nil
	}
	if taken, ok := p.lookupLocal(name.Lexeme); ok {
		return p.error(name, fmt.Sprintf("there is already a %s named %q in this scope", taken, name.Lexeme))
	}
	p.scopes[len(p.scopes)-1].kinds[name.Lexeme] = kind
	return nil
}

// lookupLocal returns what name is already declared as, searching outwards from
// the innermost scope and stopping at the first barrier.
func (p *parser) lookupLocal(name string) (string, bool) {
	for index := len(p.scopes) - 1; index >= 0; index-- {
		if kind, ok := p.scopes[index].kinds[name]; ok {
			return kind, true
		}
		if p.scopes[index].barrier {
			break
		}
	}
	return "", false
}

// declareParameters records a parameter list in the scope the body will be read
// in, which is where Godot puts them: parse_function_signature adds each one to
// the suite it is about to parse. It holds a parameter only against the others of
// its own list, through the function's parameters_indices, and adds it to the
// suite unchecked, so a lambda's parameter may carry a name the function around
// it already uses. The scope is the one just opened for the function, which holds
// nothing but the parameters read so far.
func (p *parser) declareParameters(parameters []ast.Parameter) error {
	if len(p.scopes) == 0 {
		return nil
	}
	own := p.scopes[len(p.scopes)-1].kinds
	for _, parameter := range parameters {
		if _, ok := own[parameter.Name]; ok {
			name := token.Token{Type: token.Identifier, Lexeme: parameter.Name, Span: parameter.NameSpan}
			return p.error(name, fmt.Sprintf("there is already a %s named %q in this scope", parameterName, parameter.Name))
		}
		own[parameter.Name] = parameterName
	}
	return nil
}

// pushClassScope opens the member table of a class. A class's members are kept
// apart from the locals of its functions, so a local may carry a member's name.
func (p *parser) pushClassScope() {
	p.classScopes = append(p.classScopes, map[string]string{})
}

func (p *parser) popClassScope() {
	p.classScopes = p.classScopes[:len(p.classScopes)-1]
}

// declareMember records a member of the class being read, or reports the member
// that already carries the name. An unnamed member, which is what an anonymous
// enum is, declares nothing.
func (p *parser) declareMember(name token.Token, kind string) error {
	if len(p.classScopes) == 0 || name.Lexeme == "" {
		return nil
	}
	members := p.classScopes[len(p.classScopes)-1]
	if taken, ok := members[name.Lexeme]; ok {
		return p.error(name, fmt.Sprintf("%s %q has the same name as a previously declared %s", kind, name.Lexeme, taken))
	}
	members[name.Lexeme] = kind
	return nil
}
