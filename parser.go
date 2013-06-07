package xmlpath

import (
	"encoding/xml"
	"io"
)

type Node struct {
	kind nodeKind
	name xml.Name
	attr string
	text []byte

	nodes []Node
	pos   int
	end   int

	up   *Node
	down []*Node
}

type nodeKind int

const (
	anyNode nodeKind = iota
	startNode
	endNode
	attrNode
	textNode
	commentNode
	procInstNode
)

func Parse(r io.Reader) (*Node, error) {
	return ParseDecoder(xml.NewDecoder(r))
}

func ParseDecoder(d *xml.Decoder) (*Node, error) {
	var nodes []Node
	var text []byte

	// The root node.
	nodes = append(nodes, Node{kind: startNode})

	for {
		t, err := d.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}
		switch t := t.(type) {
		case xml.EndElement:
			nodes = append(nodes, Node{
				kind: endNode,
			})
		case xml.StartElement:
			nodes = append(nodes, Node{
				kind: startNode,
				name: t.Name,
			})
			for _, attr := range t.Attr {
				nodes = append(nodes, Node{
					kind: attrNode,
					name: attr.Name,
					attr: attr.Value,
				})
			}
		case xml.CharData:
			texti := len(text)
			text = append(text, t...)
			nodes = append(nodes, Node{
				kind: textNode,
				text: text[texti : texti+len(t)],
			})
		case xml.Comment:
			texti := len(text)
			text = append(text, t...)
			nodes = append(nodes, Node{
				kind: commentNode,
				text: text[texti : texti+len(t)],
			})
		case xml.ProcInst:
			texti := len(text)
			text = append(text, t.Inst...)
			nodes = append(nodes, Node{
				kind: procInstNode,
				name: xml.Name{Local: t.Target},
				text: text[texti : texti+len(t.Inst)],
			})
		}
	}

	// Close the root node.
	nodes = append(nodes, Node{kind: endNode})

	stack := make([]*Node, 0, len(nodes))
	downs := make([]*Node, len(nodes))
	downCount := 0

	for pos := range nodes {

		switch nodes[pos].kind {

		case startNode, attrNode, textNode, commentNode, procInstNode:
			node := &nodes[pos]
			node.nodes = nodes
			node.pos = pos
			if len(stack) > 0 {
				node.up = stack[len(stack)-1]
			}
			if node.kind == startNode {
				stack = append(stack, node)
			} else {
				node.end = pos + 1
			}

		case endNode:
			node := stack[len(stack)-1]
			node.end = pos
			stack = stack[:len(stack)-1]

			// Compute downs. Doing that here is what enables the
			// use of a slice of a contiguous pre-allocated block.
			node.down = downs[downCount:downCount]
			for i := node.pos + 1; i < node.end; i++ {
				if nodes[i].up == node {
					switch nodes[i].kind {
					case startNode, textNode, commentNode, procInstNode:
						node.down = append(node.down, &nodes[i])
						downCount++
					}
				}
			}
			if len(stack) == 0 {
				return node, nil
			}
		}
	}
	return nil, io.EOF
}
