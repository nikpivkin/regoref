package regoref

import (
	"errors"
	"fmt"
	"iter"
	"slices"

	"gopkg.in/yaml.v3"
)

type OrderedMap struct {
	keys []string
	data map[string]any
}

func (m *OrderedMap) Get(key string) (any, bool) {
	val, exists := m.data[key]
	return val, exists
}

func (m *OrderedMap) Set(key string, value any) {
	if _, exists := m.data[key]; !exists {
		m.keys = append(m.keys, key)
	}
	m.data[key] = value
}

func (om *OrderedMap) InsertAfter(afterKey, key string, value any) {
	om.keys = slices.DeleteFunc(om.keys, func(k string) bool {
		return k == key
	})

	index := slices.Index(om.keys, afterKey)

	if index == -1 {
		om.keys = append(om.keys, key)
	} else {
		om.keys = append(om.keys[:index+1], append([]string{key}, om.keys[index+1:]...)...)
	}
	om.data[key] = value
}

func (om *OrderedMap) MoveAfter(afterKey, key string) {
	if _, ok := om.data[key]; !ok {
		return
	}
	om.InsertAfter(afterKey, key, om.data[key])
}

func (om *OrderedMap) Remove(key string) {
	delete(om.data, key)
	om.keys = slices.DeleteFunc(om.keys, func(k string) bool {
		return k == key
	})
}

func (m *OrderedMap) Iter() iter.Seq2[string, any] {
	return func(yield func(string, any) bool) {
		for _, k := range m.keys {
			v := m.data[k]
			if !yield(k, v) {
				return
			}
		}
	}
}

func (m *OrderedMap) Unwrap() map[string]any {
	out := make(map[string]any, len(m.keys))
	for k, v := range m.data {
		switch vv := v.(type) {
		case *OrderedMap:
			out[k] = vv.Unwrap()
		default:
			out[k] = vv
		}
	}
	return out
}

func (m *OrderedMap) UnmarshalYAML(n *yaml.Node) error {
	if n.Kind != yaml.MappingNode {
		return fmt.Errorf("expected map node, got %s", n.Tag)
	}

	if len(n.Content)%2 != 0 {
		return errors.New("invalid map node content length")
	}

	size := len(n.Content) / 2
	m.keys = make([]string, 0, size)
	m.data = make(map[string]any, size)

	for i := 0; i < len(n.Content); i += 2 {
		keyNode, valueNode := n.Content[i], n.Content[i+1]

		var key string
		if err := keyNode.Decode(&key); err != nil {
			return err
		}
		m.keys = append(m.keys, key)

		value, err := parseYAMLNode(valueNode)
		if err != nil {
			return err
		}
		m.data[key] = value
	}
	return nil
}

func parseYAMLNode(node *yaml.Node) (any, error) {
	switch node.Kind {
	case yaml.MappingNode:
		var nested OrderedMap
		if err := nested.UnmarshalYAML(node); err != nil {
			return nil, err
		}
		return &nested, nil
	case yaml.SequenceNode:
		seq := make([]any, 0, len(node.Content))
		for _, item := range node.Content {
			v, err := parseYAMLNode(item)
			if err != nil {
				return nil, err
			}
			seq = append(seq, v)
		}
		return seq, nil
	default:
		var value any
		if err := node.Decode(&value); err != nil {
			return nil, err
		}
		return value, nil
	}
}

func (m OrderedMap) MarshalYAML() (any, error) {
	return m.marshalYAML()
}

func (m OrderedMap) marshalYAML() (*yaml.Node, error) {
	node := &yaml.Node{
		Kind:    yaml.MappingNode,
		Tag:     "!!map",
		Content: make([]*yaml.Node, 0, len(m.keys)),
	}

	for _, key := range m.keys {
		keyNode := &yaml.Node{
			Kind:  yaml.ScalarNode,
			Tag:   "!!str",
			Value: fmt.Sprintf("%v", key),
		}
		val := m.data[key]
		valNode, err := marshalValue(val)
		if err != nil {
			return nil, err
		}
		node.Content = append(node.Content, keyNode, valNode)
	}

	return node, nil
}

func marshalValue(v any) (*yaml.Node, error) {
	switch vv := v.(type) {
	case *OrderedMap:
		return vv.marshalYAML()
	case []any:
		return newSeqNode(vv)
	case []string:
		return newSeqNode(vv)
	case string:
		return &yaml.Node{
			Kind:  yaml.ScalarNode,
			Tag:   "!!str",
			Value: vv,
		}, nil
	default:
		if v == nil {
			return &yaml.Node{
				Kind:  yaml.ScalarNode,
				Tag:   "!!null",
				Value: "null",
			}, nil
		}
		return &yaml.Node{
			Kind:  yaml.ScalarNode,
			Tag:   inferYAMLTag(v), // "!!str", "!!int", "!!bool", ...
			Value: fmt.Sprintf("%v", v),
		}, nil
	}
}

func inferYAMLTag(v any) string {
	switch v.(type) {
	case int, int64, int32:
		return "!!int"
	case float64, float32:
		return "!!float"
	case bool:
		return "!!bool"
	case string:
		return "!!str"
	default:
		panic(fmt.Sprintf("unsupported type: %T", v))
	}
}

func newSeqNode[T any](elems []T) (*yaml.Node, error) {
	seq := &yaml.Node{
		Kind:    yaml.SequenceNode,
		Tag:     "!!seq",
		Content: make([]*yaml.Node, 0, len(elems)),
	}
	for _, el := range elems {
		enode, err := marshalValue(el)
		if err != nil {
			return nil, err
		}
		seq.Content = append(seq.Content, enode)
	}
	return seq, nil
}

func GetFromMap[V any](om *OrderedMap, key string) (V, bool) {
	var zero V
	raw, exists := om.Get(key)
	if !exists {
		return zero, false
	}

	v, ok := raw.(V)
	if ok {
		return v, true
	}
	return zero, false
}
