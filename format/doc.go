package format

import "strings"

// doc is a layout document. A document describes the text to emit together with
// the places it may be broken, and the renderer chooses which of those breaks to
// take so that lines stay inside the column budget.
type doc interface{ isDoc() }

// docText is literal output. It may contain newlines only when it reproduces
// source that already did, such as a triple-quoted string literal.
type docText struct{ text string }

// docConcat emits its parts in order.
type docConcat struct{ parts []doc }

// docGroup emits its contents on one line when they fit in the remaining
// budget, and otherwise takes every break directly inside it.
type docGroup struct {
	inner doc
	// yielding says the group is measured only as far as the next place the line
	// could break after it, rather than as far as the line must run if nothing
	// after it breaks. A yielding group so stays on one line when it fits there
	// itself, and leaves a break still needed to a group that follows it.
	yielding bool
}

// docNest adds indentation levels to the breaks inside it.
type docNest struct {
	levels int
	inner  doc
}

type lineKind int

const (
	// lineSpace is a space when flat and a newline when broken.
	lineSpace lineKind = iota
	// lineSoft is nothing when flat and a newline when broken.
	lineSoft
	// lineHard is always a newline, and forces every enclosing group to break.
	lineHard
)

type docLine struct{ kind lineKind }

// docIfBreak emits broken when the enclosing group breaks and flat otherwise.
type docIfBreak struct {
	broken doc
	flat   doc
}

func (docText) isDoc()    {}
func (docConcat) isDoc()  {}
func (docGroup) isDoc()   {}
func (docNest) isDoc()    {}
func (docLine) isDoc()    {}
func (docIfBreak) isDoc() {}

func text(value string) doc { return docText{text: value} }

func concat(parts ...doc) doc {
	kept := make([]doc, 0, len(parts))
	for _, part := range parts {
		if part == nil {
			continue
		}
		if empty, ok := part.(docText); ok && empty.text == "" {
			continue
		}
		kept = append(kept, part)
	}
	if len(kept) == 1 {
		return kept[0]
	}
	return docConcat{parts: kept}
}

func join(separator doc, parts []doc) doc {
	if len(parts) == 0 {
		return text("")
	}
	joined := make([]doc, 0, len(parts)*2-1)
	for index, part := range parts {
		if index > 0 {
			joined = append(joined, separator)
		}
		joined = append(joined, part)
	}
	return concat(joined...)
}

// endsWithLineComment reports whether the last thing a document emits is a
// comment. A comment runs to the end of its line, so anything concatenated
// after one is swallowed into its text instead of being emitted as code.
func endsWithLineComment(d doc) bool {
	switch node := d.(type) {
	case docText:
		// A trailing comment is emitted with the spaces that separate it from
		// the code it follows, so the marker is not necessarily first.
		return strings.HasPrefix(strings.TrimLeft(node.text, " \t"), "#")
	case docConcat:
		if len(node.parts) == 0 {
			return false
		}
		return endsWithLineComment(node.parts[len(node.parts)-1])
	case docGroup:
		return endsWithLineComment(node.inner)
	case docNest:
		return endsWithLineComment(node.inner)
	case docIfBreak:
		return endsWithLineComment(node.broken) || endsWithLineComment(node.flat)
	default:
		return false
	}
}

func group(inner doc) doc { return docGroup{inner: inner} }

func yieldingGroup(inner doc) doc { return docGroup{inner: inner, yielding: true} }

func nest(levels int, inner doc) doc { return docNest{levels: levels, inner: inner} }

func ifBroken(broken, flat doc) doc { return docIfBreak{broken: broken, flat: flat} }

var (
	spaceLine = docLine{kind: lineSpace}
	softLine  = docLine{kind: lineSoft}
	hardLine  = docLine{kind: lineHard}
)

type renderMode int

const (
	modeFlat renderMode = iota
	modeBreak
)

type command struct {
	indent int
	mode   renderMode
	doc    doc
}

// render lays out document within the option's column budget.
func render(document doc, options Options) string {
	var output []byte
	column := 0
	stack := []command{{mode: modeBreak, doc: document}}
	for len(stack) > 0 {
		current := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		switch node := current.doc.(type) {
		case docText:
			output = append(output, node.text...)
			if index := strings.LastIndexByte(node.text, '\n'); index >= 0 {
				column = textWidth(node.text[index+1:], options)
			} else {
				column += textWidth(node.text, options)
			}
		case docConcat:
			for index := len(node.parts) - 1; index >= 0; index-- {
				stack = append(stack, command{indent: current.indent, mode: current.mode, doc: node.parts[index]})
			}
		case docNest:
			stack = append(stack, command{indent: current.indent + node.levels, mode: current.mode, doc: node.inner})
		case docGroup:
			flat := command{indent: current.indent, mode: modeFlat, doc: node.inner}
			if current.mode == modeFlat || fits(flat, stack, options.LineWidth-column, node.yielding, options) {
				stack = append(stack, flat)
				continue
			}
			stack = append(stack, command{indent: current.indent, mode: modeBreak, doc: node.inner})
		case docIfBreak:
			chosen := node.flat
			if current.mode == modeBreak {
				chosen = node.broken
			}
			stack = append(stack, command{indent: current.indent, mode: current.mode, doc: chosen})
		case docLine:
			if current.mode == modeFlat && node.kind != lineHard {
				if node.kind == lineSpace {
					output = append(output, ' ')
					column++
				}
				continue
			}
			output = append(trimTrailingSpace(output), '\n')
			output = append(output, strings.Repeat(options.indentUnit(), current.indent)...)
			column = current.indent * options.indentColumns()
		}
	}
	return string(trimTrailingSpace(output))
}

// fits reports whether next, followed by the already-queued rest, reaches a
// break or ends within width columns. A group in the rest is taken to stay on
// one line, unless yielding says that a group free to break counts as a break.
func fits(next command, rest []command, width int, yielding bool, options Options) bool {
	remaining := width
	stack := []command{next}
	restIndex := len(rest)
	// inRest reports that next has been measured in full, so that everything
	// still to measure comes after it.
	inRest := false
	for remaining >= 0 {
		if len(stack) == 0 {
			if restIndex == 0 {
				return true
			}
			inRest = true
			restIndex--
			stack = append(stack, rest[restIndex])
			continue
		}
		current := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		switch node := current.doc.(type) {
		case docText:
			// A literal that already spans lines can never be made to fit, so the
			// enclosing group breaks instead of measuring past the newline.
			if strings.ContainsRune(node.text, '\n') {
				return false
			}
			remaining -= textWidth(node.text, options)
		case docConcat:
			for index := len(node.parts) - 1; index >= 0; index-- {
				stack = append(stack, command{indent: current.indent, mode: current.mode, doc: node.parts[index]})
			}
		case docNest:
			stack = append(stack, command{indent: current.indent + node.levels, mode: current.mode, doc: node.inner})
		case docGroup:
			mode := modeFlat
			if yielding && inRest {
				mode = current.mode
			}
			stack = append(stack, command{indent: current.indent, mode: mode, doc: node.inner})
		case docIfBreak:
			chosen := node.flat
			if current.mode == modeBreak {
				chosen = node.broken
			}
			stack = append(stack, command{indent: current.indent, mode: current.mode, doc: chosen})
		case docLine:
			if current.mode == modeBreak {
				return true
			}
			switch node.kind {
			case lineHard:
				return false
			case lineSpace:
				remaining--
			}
		}
	}
	return false
}

// textWidth returns the column width of value, expanding tabs.
func textWidth(value string, options Options) int {
	width := 0
	for _, character := range value {
		if character == '\t' {
			width += options.TabWidth
			continue
		}
		width++
	}
	return width
}

// trimTrailingSpace removes the tabs and spaces that end output, which is done
// wherever the renderer ends a line, so that an indented blank line carries no
// whitespace. A line break inside a literal is not the renderer's, so whitespace
// before one is part of the literal's value and is never looked at.
func trimTrailingSpace(output []byte) []byte {
	end := len(output)
	for end > 0 && (output[end-1] == ' ' || output[end-1] == '\t') {
		end--
	}
	return output[:end]
}
