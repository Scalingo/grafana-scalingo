package extract

import (
	"io"

	jsoniter "github.com/json-iterator/go"
)

func logf(format string, a ...interface{}) {
	//fmt.Printf(format, a...)
}

// nolint:gocyclo
// ReadDashboard will take a byte stream and return dashboard info
func ReadDashboard(stream io.Reader, lookup DatasourceLookup) *DashboardInfo {
	iter := jsoniter.Parse(jsoniter.ConfigDefault, stream, 1024)
	dash := &DashboardInfo{}

	for l1Field := iter.ReadObject(); l1Field != ""; l1Field = iter.ReadObject() {
		// Skip null values so we don't need special int handling
		if iter.WhatIsNext() == jsoniter.NilValue {
			iter.Skip()
			continue
		}

		switch l1Field {
		case "id":
			dash.ID = iter.ReadInt64()

		case "uid":
			dash.UID = iter.ReadString()

		case "title":
			dash.Title = iter.ReadString()

		case "description":
			dash.Description = iter.ReadString()

		case "schemaVersion":
			dash.SchemaVersion = iter.ReadInt64()

		case "timezone":
			dash.TimeZone = iter.ReadString()

		case "editable":
			dash.ReadOnly = !iter.ReadBool()

		case "refresh":
			nxt := iter.WhatIsNext()
			if nxt == jsoniter.StringValue {
				dash.Refresh = iter.ReadString()
			} else {
				iter.Skip()
			}

		case "tags":
			for iter.ReadArray() {
				dash.Tags = append(dash.Tags, iter.ReadString())
			}

		case "links":
			for iter.ReadArray() {
				iter.Skip()
				dash.LinkCount++
			}

		case "time":
			obj, ok := iter.Read().(map[string]interface{})
			if ok {
				dash.TimeFrom, _ = obj["from"].(string)
				dash.TimeTo, _ = obj["to"].(string)
			}

		case "panels":
			for iter.ReadArray() {
				dash.Panels = append(dash.Panels, readPanelInfo(iter, lookup))
			}

		case "rows":
			for iter.ReadArray() {
				v := iter.Read()
				logf("[DASHBOARD.ROW???] id=%s // %v\n", dash.UID, v)
			}

		case "annotations":
			for sub := iter.ReadObject(); sub != ""; sub = iter.ReadObject() {
				if sub == "list" {
					for iter.ReadArray() {
						v := iter.Read()
						logf("[dash.anno] %v\n", v)
					}
				} else {
					iter.Skip()
				}
			}

		case "templating":
			for sub := iter.ReadObject(); sub != ""; sub = iter.ReadObject() {
				if sub == "list" {
					for iter.ReadArray() {
						for k := iter.ReadObject(); k != ""; k = iter.ReadObject() {
							if k == "name" {
								dash.TemplateVars = append(dash.TemplateVars, iter.ReadString())
							} else {
								iter.Skip()
							}
						}
					}
				} else {
					iter.Skip()
				}
			}

		// Ignore these properties
		case "timepicker":
			fallthrough
		case "version":
			fallthrough
		case "iteration":
			iter.Skip()

		default:
			v := iter.Read()
			logf("[DASHBOARD] support key: %s / %v\n", l1Field, v)
		}
	}

	if dash.UID == "" {
		logf("All dashbaords should have a UID defined")
	}

	targets := newTargetInfo(lookup)
	for _, panel := range dash.Panels {
		targets.addPanel(panel)
	}
	dash.Datasource = targets.GetDatasourceInfo()

	return dash
}

// will always return strings for now
func readPanelInfo(iter *jsoniter.Iterator, lookup DatasourceLookup) PanelInfo {
	panel := PanelInfo{}

	targets := newTargetInfo(lookup)

	for l1Field := iter.ReadObject(); l1Field != ""; l1Field = iter.ReadObject() {
		if iter.WhatIsNext() == jsoniter.NilValue {
			if l1Field == "datasource" {
				targets.addDatasource(iter)
				continue
			}

			// Skip null values so we don't need special int handling
			iter.Skip()
			continue
		}

		switch l1Field {
		case "id":
			panel.ID = iter.ReadInt64()

		case "type":
			panel.Type = iter.ReadString()

		case "title":
			panel.Title = iter.ReadString()

		case "description":
			panel.Description = iter.ReadString()

		case "pluginVersion":
			panel.PluginVersion = iter.ReadString() // since 7x (the saved version for the plugin model)

		case "datasource":
			targets.addDatasource(iter)

		case "targets":
			for iter.ReadArray() {
				targets.addTarget(iter)
			}

		case "transformations":
			for iter.ReadArray() {
				for sub := iter.ReadObject(); sub != ""; sub = iter.ReadObject() {
					if sub == "id" {
						panel.Transformations = append(panel.Transformations, iter.ReadString())
					} else {
						iter.Skip()
					}
				}
			}

		// Rows have nested panels
		case "panels":
			for iter.ReadArray() {
				panel.Collapsed = append(panel.Collapsed, readPanelInfo(iter, lookup))
			}

		case "options":
			fallthrough

		case "gridPos":
			fallthrough

		case "fieldConfig":
			iter.Skip()

		default:
			v := iter.Read()
			logf("[PANEL] support key: %s / %v\n", l1Field, v)
		}
	}

	panel.Datasource = targets.GetDatasourceInfo()

	return panel
}
