package winreg

import "slices"

type ValueKind int

const (
	String ValueKind = iota
	DWord
	MultiString
)

type Value struct {
	name   string
	kind   ValueKind
	text   string
	number uint32
	items  []string
}

func StringValue(name, text string) Value {
	return Value{name: name, kind: String, text: text}
}

func DWordValue(name string, number uint32) Value {
	return Value{name: name, kind: DWord, number: number}
}

func MultiStringValue(name string, items ...string) Value {
	return Value{name: name, kind: MultiString, items: slices.Clone(items)}
}

func (value Value) Name() string {
	return value.name
}

func (value Value) Kind() ValueKind {
	return value.kind
}

func (value Value) Text() string {
	return value.text
}

func (value Value) Number() uint32 {
	return value.number
}

func (value Value) Items() []string {
	return slices.Clone(value.items)
}

type Key struct {
	path   string
	values []Value
}

func NewKey(path string, values ...Value) Key {
	return Key{path: path, values: slices.Clone(values)}
}

func (key Key) Path() string {
	return key.path
}

func (key Key) Values() []Value {
	return slices.Clone(key.values)
}

type Patch struct {
	keys []Key
}

func NewPatch(keys ...Key) Patch {
	return Patch{keys: slices.Clone(keys)}
}

func (patch Patch) Keys() []Key {
	return slices.Clone(patch.keys)
}
