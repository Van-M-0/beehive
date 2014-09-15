package bh

import "errors"

// A simple dictionary that uses in memory maps.
type inMemoryState struct {
	Name  string
	Dicts map[DictName]*inMemDict
	tx    *inMemoryTx
}

func (s *inMemoryState) BeginTx() error {
	if s.tx != nil {
		return errors.New("Transaction is already started")
	}

	s.tx = s.newTransaction()
	return nil
}

func (s *inMemoryState) newTransaction() *inMemoryTx {
	return &inMemoryTx{
		state: s,
		stage: make(map[DictName]*inMemStagedDict),
	}
}

func (s *inMemoryState) Tx() []StateOp {
	if s.tx == nil {
		return nil
	}

	l := 0
	for _, dict := range s.tx.stage {
		l += len(dict.ops)
	}

	ops := make([]StateOp, l)
	for _, dict := range s.tx.stage {
		for _, op := range dict.ops {
			ops = append(ops, op)
		}
	}
	return ops
}

func (s *inMemoryState) CommitTx() error {
	if s.tx == nil {
		return errors.New("No active transaction")
	}

	s.tx.Commit()
	return nil
}

func (s *inMemoryState) AbortTx() error {
	if s.tx == nil {
		return errors.New("No active transaction")
	}

	s.tx.Abort()
	return nil
}

func (s *inMemoryState) Dict(name DictName) Dict {
	if s.tx != nil {
		return s.tx.Dict(name)
	}

	return s.inMemDict(name)
}

func (s *inMemoryState) inMemDict(name DictName) *inMemDict {
	d, ok := s.Dicts[name]
	if !ok {
		d = &inMemDict{name, make(map[Key]Value)}
		s.Dicts[name] = d
	}
	return d
}

type inMemDict struct {
	DictName DictName
	Dict     map[Key]Value
}

func (d inMemDict) Name() DictName {
	return d.DictName
}

func (d *inMemDict) Get(k Key) (Value, error) {
	v, ok := d.Dict[k]
	if !ok {
		return v, errors.New("Key does not exist.")
	}
	return v, nil
}

func (d *inMemDict) Put(k Key, v Value) error {
	d.Dict[k] = v
	return nil
}

func (d *inMemDict) Del(k Key) error {
	delete(d.Dict, k)
	return nil
}

func (d *inMemDict) ForEach(f IterFn) {
	for k, v := range d.Dict {
		f(k, v)
	}
}

type inMemoryTx struct {
	state *inMemoryState
	stage map[DictName]*inMemStagedDict
}

type inMemStagedDict struct {
	dict *inMemDict
	ops  map[Key]StateOp
}

func (t *inMemoryTx) Dict(n DictName) Dict {
	d := &inMemStagedDict{
		dict: t.state.inMemDict(n),
		ops:  make(map[Key]StateOp),
	}

	t.stage[n] = d
	return d
}

func (t *inMemoryTx) Commit() {
	for _, d := range t.stage {
		for _, o := range d.ops {
			switch o.T {
			case Put:
				d.dict.Put(o.K, o.V)
			case Del:
				d.dict.Del(o.K)
			}
		}
	}
}

func (t *inMemoryTx) Abort() {
	return
}

func (d *inMemStagedDict) Name() DictName {
	return d.dict.Name()
}

func (d *inMemStagedDict) Put(k Key, v Value) error {
	d.ops[k] = StateOp{
		T: Put,
		D: d.dict.Name(),
		K: k,
		V: v,
	}
	return nil
}

func (d *inMemStagedDict) Get(k Key) (Value, error) {
	op, ok := d.ops[k]
	if ok {
		switch op.T {
		case Put:
			return op.V, nil
		case Del:
			return nil, errors.New("No such key")
		}
	}

	return d.dict.Get(k)
}

func (d *inMemStagedDict) Del(k Key) error {
	d.ops[k] = StateOp{
		T: Del,
		D: d.dict.Name(),
		K: k,
	}
	return nil
}

func (d *inMemStagedDict) ForEach(f IterFn) {
	d.dict.ForEach(func(k Key, v Value) {
		op, ok := d.ops[k]
		if ok {
			switch op.T {
			case Put:
				f(op.K, op.V)
				return
			case Del:
				return
			}
		}

		f(k, v)
	})
}
