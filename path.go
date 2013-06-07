package xmlpath

import (
	"fmt"
	"unicode/utf8"
)

type Path struct {
	path  string
	steps []pathStep
}

func (p *Path) Iter(node *Node) *Iter {
	iter := Iter{
		make([]pathStepState, len(p.steps)),
		make([]bool, len(node.nodes)),
	}
	for i := range p.steps {
		iter.state[i].step = &p.steps[i]
	}
	iter.state[0].init(node)
	return &iter
}

func (p *Path) Exists(node *Node) bool {
	return p.Iter(node).Next()
}

func (p *Path) String(node *Node) (s string, ok bool) {
	iter := p.Iter(node)
	if iter.Next() {
		return iter.string(), true
	}
	return "", false
}

func (p *Path) Strings(node *Node) (ss []string, ok bool) {
	iter := p.Iter(node)
	for iter.Next() {
		ss = append(ss, iter.string())
	}
	return ss, len(ss) > 0
}

type Iter struct {
	state []pathStepState
	seen  []bool
}

func (iter *Iter) string() string {
	state := &iter.state[len(iter.state)-1]
	if state.node.kind == attrNode {
		return state.node.attr
	}
	var text []byte
	for i := state.node.pos; i < state.node.end; i++ {
		node := &state.node.nodes[i]
		if node.kind == textNode || node.kind == commentNode || node.kind == procInstNode {
			text = append(text, node.text...)
		}
	}
	return string(text)
}

func (iter *Iter) Node() *Node {
	return iter.state[len(iter.state)-1].node
}

func (iter *Iter) Next() bool {
	tip := len(iter.state) - 1
outer:
	for {
		for !iter.state[tip].next() {
			tip--
			if tip == -1 {
				return false
			}
		}
		for tip < len(iter.state)-1 {
			tip++
			iter.state[tip].init(iter.state[tip-1].node)
			if !iter.state[tip].next() {
				tip--
				continue outer
			}
		}
		if iter.seen[iter.state[tip].node.pos] {
			continue
		}
		iter.seen[iter.state[tip].node.pos] = true
		return true
	}
	panic("unreachable")
}

type pathStepState struct {
	step *pathStep
	node *Node
	idx  int
	aux  int
}

func (s *pathStepState) init(node *Node) {
	s.node = node
	s.idx = 0
	s.aux = 0
}

func (s *pathStepState) next() bool {
	if s.node == nil {
		return false
	}
	if s.step.root && s.idx == 0 {
		for s.node.up != nil {
			s.node = s.node.up
		}
	}

	switch s.step.axis {

	case "self":
		if s.idx == 0 && s.step.match(s.node) {
			s.idx++
			return true
		}

	case "parent":
		if s.idx == 0 && s.node.up != nil && s.step.match(s.node.up) {
			s.idx++
			s.node = s.node.up
			return true
		}

	case "ancestor", "ancestor-or-self":
		if s.idx == 0 && s.step.axis == "ancestor-or-self" {
			s.idx++
			if s.step.match(s.node) {
				return true
			}
		}
		for s.node.up != nil {
			s.node = s.node.up
			s.idx++
			if s.step.match(s.node) {
				return true
			}
		}

	case "child":
		var down []*Node
		if s.idx == 0 {
			down = s.node.down
		} else {
			down = s.node.up.down
		}
		for s.idx < len(down) {
			node := down[s.idx]
			s.idx++
			if s.step.match(node) {
				s.node = node
				return true
			}
		}

	case "descendant", "descendant-or-self":
		if s.idx == 0 {
			s.idx = s.node.pos
			s.aux = s.node.end
			if s.step.axis == "descendant" {
				s.idx++
			}
		}
		for s.idx < s.aux {
			node := &s.node.nodes[s.idx]
			s.idx++
			if node.kind == attrNode {
				continue
			}
			if s.step.match(node) {
				s.node = node
				return true
			}
		}

	case "following":
		if s.idx == 0 {
			s.idx = s.node.end
		}
		for s.idx < len(s.node.nodes) {
			node := &s.node.nodes[s.idx]
			s.idx++
			if node.kind == attrNode {
				continue
			}
			if s.step.match(node) {
				s.node = node
				return true
			}
		}

	case "following-sibling":
		var down []*Node
		if s.node.up != nil {
			down = s.node.up.down
			if s.idx == 0 {
				for s.idx < len(down) {
					node := down[s.idx]
					s.idx++
					if node == s.node {
						break
					}
				}
			}
		}
		for s.idx < len(down) {
			node := down[s.idx]
			s.idx++
			if s.step.match(node) {
				s.node = node
				return true
			}
		}

	case "preceding":
		if s.idx == 0 {
			s.aux = s.node.pos // Detect ancestors.
			s.idx = s.node.pos - 1
		}
		for s.idx >= 0 {
			node := &s.node.nodes[s.idx]
			s.idx--
			if node.kind == attrNode {
				continue
			}
			if node == s.node.nodes[s.aux].up {
				s.aux = s.node.nodes[s.aux].up.pos
				continue
			}
			if s.step.match(node) {
				s.node = node
				return true
			}
		}

	case "preceding-sibling":
		var down []*Node
		if s.node.up != nil {
			down = s.node.up.down
			if s.aux == 0 {
				s.aux = 1
				for s.idx < len(down) {
					node := down[s.idx]
					s.idx++
					if node == s.node {
						s.idx--
						break
					}
				}
			}
		}
		for s.idx >= 0 {
			node := down[s.idx]
			s.idx--
			if s.step.match(node) {
				s.node = node
				return true
			}
		}

	case "attribute":
		if s.idx == 0 {
			s.idx = s.node.pos + 1
			s.aux = s.node.end
		}
		for s.idx < s.aux {
			node := &s.node.nodes[s.idx]
			s.idx++
			if node.kind != attrNode {
				break
			}
			if s.step.match(node) {
				s.node = node
				return true
			}
		}

	}

	s.node = nil
	return false
}

type pathStep struct {
	root bool
	axis string
	name string
	kind nodeKind
	//pred pathPredicate
	//expr expr
}

func (step *pathStep) match(node *Node) bool {
	return node.kind != endNode &&
		(step.kind == anyNode || step.kind == node.kind) &&
		(step.name == "*" || node.name.Local == step.name)
}

type pathPredicate interface {
	test(*Node) bool
}

func MustCompile(path string) *Path {
	e, err := Compile(path)
	if err != nil {
		panic(err)
	}
	return e
}

func Compile(path string) (*Path, error) {
	c := pathCompiler{nil, path, 0}
	err := c.compile()
	if err != nil {
		return nil, err
	}
	return &Path{path, c.steps}, nil
}

type pathCompiler struct {
	steps []pathStep
	path  string
	i     int
}

func (c *pathCompiler) errorf(format string, args ...interface{}) error {
	return fmt.Errorf("compiling xml path %q:%d: %s", c.path, c.i, fmt.Sprintf(format, args...))
}

func (c *pathCompiler) compile() error {
	for c.i < len(c.path) {
		step := pathStep{axis: "child"}

		if c.i == 0 && c.skipByte('/') {
			step.root = true
			if len(c.path) == 1 {
				step.name = "*"
			}
		}
		if c.peekByte('/') {
			step.axis = "descendant-or-self"
			step.name = "*"
		} else if c.skipByte('@') {
			mark := c.i
			if !c.skipName() {
				return c.errorf("missing name after @")
			}
			step.axis = "attribute"
			step.name = c.path[mark:c.i]
			step.kind = attrNode
		} else {
			mark := c.i
			if c.skipName() {
				step.name = c.path[mark:c.i]
			}
			if step.name == "" {
				return c.errorf("missing name")
			} else if step.name == "*" {
				step.kind = startNode
			} else if step.name == "." {
				step.axis = "self"
				step.name = "*"
			} else if step.name == ".." {
				step.axis = "parent"
				step.name = "*"
			} else {
				if c.skipByte(':') {
					if !c.skipByte(':') {
						return c.errorf("missing ':'")
					}
					switch step.name {
					case "attribute":
						step.kind = attrNode
					case "self", "child", "parent":
					case "descendant", "descendant-or-self":
					case "ancestor", "ancestor-or-self":
					case "following", "following-sibling":
					case "preceding", "preceding-sibling":
					default:
						return c.errorf("unsupported axis: %q", step.name)
					}
					step.axis = step.name

					mark = c.i
					if !c.skipName() {
						return c.errorf("missing name")
					}
					step.name = c.path[mark:c.i]
				}
				if c.skipByte('(') {
					conflict := step.kind != anyNode
					switch step.name {
					case "node":
						// must be anyNode
					case "text":
						step.kind = textNode
					case "comment":
						step.kind = commentNode
					case "processing-instruction":
						step.kind = procInstNode
					default:
						return c.errorf("unsupported expression: %s()", step.name)
					}
					if conflict {
						return c.errorf("%s() cannot succeed on axis %q", step.name, step.axis)
					}

					if c.skipByte('"') {
						if step.kind != procInstNode {
							return c.errorf("%s() has no arguments", step.name)
						}
						mark := c.i
						c.skipName()
						step.name = c.path[mark:c.i]
						if !c.skipByte('"') {
							return c.errorf(`missing "`)
						}
					} else {
						step.name = "*"
					}
					if !c.skipByte(')') {
						return c.errorf("missing )")
					}
				} else if step.name == "*" && step.kind == anyNode {
					step.kind = startNode
				}
			}
		}
		if c.i < len(c.path) && !c.skipByte('/') {
			return c.errorf("expected '/' but got %q", c.path[c.i])
		}
		//fmt.Printf("step: %#v\n", step)
		c.steps = append(c.steps, step)
	}
	return nil
}

func (c *pathCompiler) skipByte(b byte) bool {
	if c.i < len(c.path) && c.path[c.i] == b {
		c.i++
		return true
	}
	return false
}

func (c *pathCompiler) peekByte(b byte) bool {
	return c.i < len(c.path) && c.path[c.i] == b
}

func (c *pathCompiler) skipName() bool {
	if c.i >= len(c.path) {
		return false
	}
	if c.path[c.i] == '*' {
		c.i++
		return true
	}
	start := c.i
	for c.i < len(c.path) && (c.path[c.i] >= utf8.RuneSelf || isNameByte(c.path[c.i])) {
		c.i++
	}
	return c.i > start
}

func isNameByte(c byte) bool {
	return 'A' <= c && c <= 'Z' || 'a' <= c && c <= 'z' || '0' <= c && c <= '9' || c == '_' || c == '.' || c == '-'
}
