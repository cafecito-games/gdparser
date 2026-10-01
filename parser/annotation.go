package parser

import (
	"fmt"

	"github.com/cafecito-games/gdparser/ast"
)

// annotationTarget is what an annotation may be written against, mirroring
// AnnotationInfo::TargetKind in Godot's modules/gdscript/gdscript_parser.h.
type annotationTarget uint16

const (
	// targetScript belongs to the script as a whole and so must come before
	// "extends" and "class_name".
	targetScript annotationTarget = 1 << iota
	targetClass
	targetVariable
	targetConstant
	targetSignal
	targetFunction
	// targetStatement decorates a statement inside a function body.
	targetStatement
	// targetStandalone decorates nothing and stands on a line of its own.
	targetStandalone
)

// targetClassLevel is everything a class body may hold, which is the set Godot
// passes while reading one.
const targetClassLevel = targetClass | targetVariable | targetConstant | targetSignal | targetFunction

// annotationTargets is every annotation Godot knows, with what each may be
// written against. It mirrors the register_annotation calls in
// GDScriptParser::parse_annotation's setup, as of Godot 4.7. An annotation absent
// here is one Godot rejects outright, so the table has to be refreshed when Godot
// adds one.
var annotationTargets = map[string]annotationTarget{
	"tool":          targetScript,
	"icon":          targetScript,
	"static_unload": targetScript,
	"abstract":      targetScript | targetClass | targetFunction,

	"onready": targetVariable,

	"export":                     targetVariable,
	"export_enum":                targetVariable,
	"export_file":                targetVariable,
	"export_file_path":           targetVariable,
	"export_dir":                 targetVariable,
	"export_global_file":         targetVariable,
	"export_global_dir":          targetVariable,
	"export_multiline":           targetVariable,
	"export_placeholder":         targetVariable,
	"export_range":               targetVariable,
	"export_exp_easing":          targetVariable,
	"export_color_no_alpha":      targetVariable,
	"export_node_path":           targetVariable,
	"export_flags":               targetVariable,
	"export_flags_2d_render":     targetVariable,
	"export_flags_2d_physics":    targetVariable,
	"export_flags_2d_navigation": targetVariable,
	"export_flags_3d_render":     targetVariable,
	"export_flags_3d_physics":    targetVariable,
	"export_flags_3d_navigation": targetVariable,
	"export_flags_avoidance":     targetVariable,
	"export_storage":             targetVariable,
	"export_custom":              targetVariable,
	"export_tool_button":         targetVariable,

	"export_category": targetStandalone,
	"export_group":    targetStandalone,
	"export_subgroup": targetStandalone,

	"warning_ignore":         targetClassLevel | targetStatement,
	"warning_ignore_start":   targetStandalone,
	"warning_ignore_restore": targetStandalone,

	"rpc": targetFunction,
}

// retiredAnnotations are the three names Godot answers with the documentation
// comment that replaced them, rather than with "unrecognized".
var retiredAnnotations = map[string]string{
	"deprecated":   `"## @deprecated: Reason here."`,
	"experimental": `"## @experimental: Reason here."`,
	"tutorial":     `"## @tutorial(Title): https://example.com"`,
}

// checkAnnotation reports whether the annotation exists and may be written where
// it was, given the targets that position allows.
func (p *parser) checkAnnotation(annotation *ast.Annotation, allowed annotationTarget) error {
	name := nameToken(annotation.Name, annotation.NameSpan)
	targets, known := annotationTargets[annotation.Name]
	if !known {
		if replacement, retired := retiredAnnotations[annotation.Name]; retired {
			return p.error(name, fmt.Sprintf("the %q annotation does not exist; use %s instead",
				"@"+annotation.Name, replacement))
		}
		return p.error(name, fmt.Sprintf("unrecognized annotation %q", "@"+annotation.Name))
	}
	if targets&allowed != 0 {
		return nil
	}
	if targets&targetScript != 0 {
		// Godot names the ordering rule rather than the level, since an
		// annotation of the script is only out of place for having come late.
		return p.error(name, fmt.Sprintf("the %q annotation belongs at the top of the script, before extends and class_name",
			"@"+annotation.Name))
	}
	return p.error(name, fmt.Sprintf("the %q annotation is not allowed at this level", "@"+annotation.Name))
}

// annotationTargetOf returns what kind of declaration statement is, so that the
// annotations ahead of it can be checked against it.
func annotationTargetOf(statement ast.Statement) (annotationTarget, bool) {
	switch declaration := statement.(type) {
	case *ast.VariableDeclaration:
		if declaration.Constant {
			return targetConstant, true
		}
		return targetVariable, true
	case *ast.FunctionDeclaration:
		return targetFunction, true
	case *ast.ClassDeclaration:
		return targetClass, true
	case *ast.SignalDeclaration:
		return targetSignal, true
	case *ast.EnumDeclaration:
		// Godot registers no annotation against an enum, and passes NONE while
		// reading one, so every annotation ahead of one is out of place.
		return 0, true
	}
	return 0, false
}

// decoratesNothing reports whether an annotation belongs to no declaration below
// it. Godot handles these where it reads them rather than holding them for the
// next member: the export group markers and the warning regions stand alone, and
// an annotation of the script itself belongs to the script. An annotation that may
// also apply to a class waits, since it cannot be told yet whether it decorates
// the file or an inner class.
func decoratesNothing(name string) bool {
	targets, known := annotationTargets[name]
	if !known || targets&targetClass != 0 {
		return false
	}
	return targets&(targetStandalone|targetScript) != 0
}

// sharesItsLine reports whether an annotation that decorates nothing may be
// followed on its line by the next statement. An annotation of the script itself
// may: parse_program applies it where it reads it and asks for nothing after it,
// while a standalone one must end its line.
func sharesItsLine(name string) bool {
	return annotationTargets[name]&targetScript != 0
}

// checkAnnotationTargets reports whether every annotation ahead of statement may
// decorate it. A statement that is no declaration takes none of them, and Godot
// lets them stand on their own instead.
func (p *parser) checkAnnotationTargets(statement ast.Statement, annotations []*ast.Annotation) error {
	target, isDeclaration := annotationTargetOf(statement)
	if !isDeclaration {
		return nil
	}
	for _, annotation := range annotations {
		if err := p.checkAnnotationTarget(annotation, target); err != nil {
			return err
		}
	}
	return nil
}

// checkAnnotationTarget reports whether annotation may decorate a declaration of
// the given kind, which is the second question Godot asks: the first is whether
// the annotation belongs at that level at all.
func (p *parser) checkAnnotationTarget(annotation *ast.Annotation, target annotationTarget) error {
	if targets, known := annotationTargets[annotation.Name]; known && targets&target != 0 {
		return nil
	}
	return p.error(nameToken(annotation.Name, annotation.NameSpan),
		fmt.Sprintf("the %q annotation is not allowed at this level", "@"+annotation.Name))
}
