package schemaloader

import (
	"encoding/json"
	"fmt"

	"github.com/grafana/grafana"
	"github.com/grafana/grafana/pkg/components/simplejson"
	"github.com/grafana/grafana/pkg/schema"
	"github.com/grafana/grafana/pkg/schema/load"
	"github.com/grafana/grafana/pkg/services/featuremgmt"

	"github.com/grafana/grafana/pkg/infra/log"
)

const ServiceName = "SchemaLoader"

var baseLoadPath load.BaseLoadPaths = load.BaseLoadPaths{
	BaseCueFS:       grafana.CoreSchema,
	DistPluginCueFS: grafana.PluginSchema,
}

type RenderUser struct {
	OrgID   int64
	UserID  int64
	OrgRole string
}

func ProvideService(features featuremgmt.FeatureToggles) (*SchemaLoaderService, error) {
	dashFam, err := load.BaseDashboardFamily(baseLoadPath)
	if err != nil {
		return nil, fmt.Errorf("failed to load dashboard cue schema from path %q: %w", baseLoadPath, err)
	}
	s := &SchemaLoaderService{
		features:   features,
		DashFamily: dashFam,
		log:        log.New("schemaloader"),
	}
	return s, nil
}

type SchemaLoaderService struct {
	log        log.Logger
	DashFamily schema.VersionedCueSchema
	features   featuremgmt.FeatureToggles
}

func (rs *SchemaLoaderService) IsDisabled() bool {
	if rs.features == nil {
		return true
	}
	return !rs.features.IsEnabled(featuremgmt.FlagTrimDefaults)
}

func (rs *SchemaLoaderService) DashboardApplyDefaults(input *simplejson.Json) (*simplejson.Json, error) {
	val, _ := input.Map()
	val = removeNils(val)
	data, _ := json.Marshal(val)
	dsSchema := schema.Find(rs.DashFamily, schema.Latest())
	result, err := schema.ApplyDefaults(schema.Resource{Value: data}, dsSchema.CUE())
	if err != nil {
		return input, err
	}
	output, err := simplejson.NewJson([]byte(result.Value.(string)))
	if err != nil {
		return input, err
	}
	return output, nil
}

func (rs *SchemaLoaderService) DashboardTrimDefaults(input simplejson.Json) (simplejson.Json, error) {
	val, _ := input.Map()
	val = removeNils(val)
	data, _ := json.Marshal(val)

	dsSchema, err := schema.SearchAndValidate(rs.DashFamily, string(data))
	if err != nil {
		return input, err
	}

	result, err := schema.TrimDefaults(schema.Resource{Value: data}, dsSchema.CUE())
	if err != nil {
		return input, err
	}
	output, err := simplejson.NewJson([]byte(result.Value.(string)))
	if err != nil {
		return input, err
	}
	return *output, nil
}

func removeNils(initialMap map[string]interface{}) map[string]interface{} {
	withoutNils := map[string]interface{}{}
	for key, value := range initialMap {
		_, ok := value.(map[string]interface{})
		if ok {
			value = removeNils(value.(map[string]interface{}))
			withoutNils[key] = value
			continue
		}
		_, ok = value.([]interface{})
		if ok {
			value = removeNilArray(value.([]interface{}))
			withoutNils[key] = value
			continue
		}
		if value != nil {
			if val, ok := value.(string); ok {
				if val == "" {
					continue
				}
			}
			withoutNils[key] = value
		}
	}
	return withoutNils
}

func removeNilArray(initialArray []interface{}) []interface{} {
	withoutNils := []interface{}{}
	for _, value := range initialArray {
		_, ok := value.(map[string]interface{})
		if ok {
			value = removeNils(value.(map[string]interface{}))
			withoutNils = append(withoutNils, value)
			continue
		}
		_, ok = value.([]interface{})
		if ok {
			value = removeNilArray(value.([]interface{}))
			withoutNils = append(withoutNils, value)
			continue
		}
		if value != nil {
			if val, ok := value.(string); ok {
				if val == "" {
					continue
				}
			}
			withoutNils = append(withoutNils, value)
		}
	}
	return withoutNils
}
