package extract

import (
	jsoniter "github.com/json-iterator/go"
)

type targetInfo struct {
	lookup DatasourceLookup
	uids   map[string]*DataSourceRef
}

func newTargetInfo(lookup DatasourceLookup) targetInfo {
	return targetInfo{
		lookup: lookup,
		uids:   make(map[string]*DataSourceRef),
	}
}

func (s *targetInfo) GetDatasourceInfo() []DataSourceRef {
	keys := make([]DataSourceRef, len(s.uids))
	i := 0
	for _, v := range s.uids {
		keys[i] = *v
		i++
	}
	return keys
}

// the node will either be string (name|uid) OR ref
func (s *targetInfo) addDatasource(iter *jsoniter.Iterator) {
	switch iter.WhatIsNext() {
	case jsoniter.StringValue:
		key := iter.ReadString()
		ds := s.lookup(&DataSourceRef{UID: key})
		s.addRef(ds)

	case jsoniter.NilValue:
		s.addRef(s.lookup(nil))
		iter.Skip()

	case jsoniter.ObjectValue:
		ref := &DataSourceRef{}
		iter.ReadVal(ref)
		ds := s.lookup(ref)
		s.addRef(ds)

	default:
		v := iter.Read()
		logf("[Panel.datasource.unknown] %v\n", v)
	}
}

func (s *targetInfo) addRef(ref *DataSourceRef) {
	if ref != nil && ref.UID != "" {
		s.uids[ref.UID] = ref
	}
}

func (s *targetInfo) addTarget(iter *jsoniter.Iterator) {
	for l1Field := iter.ReadObject(); l1Field != ""; l1Field = iter.ReadObject() {
		switch l1Field {
		case "datasource":
			s.addDatasource(iter)

		case "refId":
			iter.Skip()

		default:
			v := iter.Read()
			logf("[Panel.TARGET] %s=%v\n", l1Field, v)
		}
	}
}

func (s *targetInfo) addPanel(panel PanelInfo) {
	for idx, v := range panel.Datasource {
		if v.UID != "" {
			s.uids[v.UID] = &panel.Datasource[idx]
		}
	}
}
