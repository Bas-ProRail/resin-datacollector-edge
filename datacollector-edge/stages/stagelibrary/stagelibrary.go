/*
 * Copyright 2017 StreamSets Inc.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */
package stagelibrary

import (
	"errors"
	"fmt"
	"github.com/streamsets/datacollector-edge/api"
	"github.com/streamsets/datacollector-edge/container/common"
	"github.com/streamsets/datacollector-edge/container/util"
	"reflect"
	"strings"
	"sync"
)

type NewStageCreator func() api.Stage

var reg *registry

type registry struct {
	sync.RWMutex
	newStageCreatorMap map[string]NewStageCreator
	stageDefinitionMap map[string]*common.StageDefinition
}

func init() {
	reg = new(registry)
	reg.newStageCreatorMap = make(map[string]NewStageCreator)
	reg.stageDefinitionMap = make(map[string]*common.StageDefinition)
}

func SetCreator(library string, stageName string, newStageCreator NewStageCreator) {
	stageKey := library + ":" + stageName
	reg.Lock()
	reg.newStageCreatorMap[stageKey] = newStageCreator
	reg.Unlock()
}

func GetCreator(library string, stageName string) (NewStageCreator, bool) {
	stageKey := library + ":" + stageName
	reg.RLock()
	s, b := reg.newStageCreatorMap[stageKey]
	reg.RUnlock()
	return s, b
}

func CreateStageInstance(library string, stageName string) (api.Stage, *common.StageDefinition, error) {
	if t, ok := GetCreator(library, stageName); ok {
		v := t()

		stageDefinition := extractStageDefinition(library, stageName, v)
		return v, stageDefinition, nil
	} else {
		return nil, nil, errors.New("No Stage Instance found for : " + library + ", stage: " + stageName)
	}
}

func extractStageDefinition(library string, stageName string, stageInstance interface{}) *common.StageDefinition {
	stageDefinition := &common.StageDefinition{
		Name:                 stageName,
		Library:              library,
		ConfigDefinitionsMap: make(map[string]*common.ConfigDefinition),
	}
	t := reflect.TypeOf(stageInstance).Elem()
	extractConfigDefinitions(t, "", stageDefinition.ConfigDefinitionsMap)
	return stageDefinition
}

func extractConfigDefinitions(
	t reflect.Type,
	configPrefix string,
	configDefinitionsMap map[string]*common.ConfigDefinition,
) {
	for i := 0; i < t.NumField(); i++ {
		field := t.Field(i)
		configDefTag := field.Tag.Get(common.CONFIG_DEF_TAG_NAME)
		if len(configDefTag) > 0 {
			extractConfigDefinition(field, configDefTag, configPrefix, configDefinitionsMap)
		} else {
			configDefBeanTag := field.Tag.Get(common.CONFIG_DEF_BEAN_TAG_NAME)
			if len(configDefBeanTag) > 0 {
				newConfigPrefix := configPrefix + util.LcFirst(field.Name) + "."
				extractConfigDefinitions(field.Type, newConfigPrefix, configDefinitionsMap)
			}
		}
	}
}

func extractConfigDefinition(
	field reflect.StructField,
	configDefTag string,
	configPrefix string,
	configDefinitionsMap map[string]*common.ConfigDefinition,
) {
	configDef := &common.ConfigDefinition{Evaluation: common.EVALUATION_IMPLICIT}
	configDefTagValues := strings.Split(configDefTag, ",")
	for _, tagValue := range configDefTagValues {
		args := strings.Split(tagValue, "=")
		switch args[0] {
		case "type":
			fmt.Sscanf(tagValue, "type=%s", &configDef.Type)
		case "required":
			fmt.Sscanf(tagValue, "required=%t", &configDef.Required)
		case "evaluation":
			fmt.Sscanf(tagValue, "evaluation=%s", &configDef.Evaluation)
		}
	}
	configDef.Name = configPrefix + util.LcFirst(field.Name)
	configDef.FieldName = field.Name

	listBeanModelTag := field.Tag.Get(common.LIST_BEAN_MODEL_TAG_NAME)
	if len(listBeanModelTag) > 0 {
		configDefinitionsMap := make(map[string]*common.ConfigDefinition)
		extractConfigDefinitions(field.Type.Elem(), "", configDefinitionsMap)
		configDef.Model = common.ModelDefinition{
			ConfigDefinitionsMap: configDefinitionsMap,
		}
	}

	configDefinitionsMap[configDef.Name] = configDef
}
