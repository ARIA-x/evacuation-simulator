package aria_utility_settings

import (
	"encoding/json"
	"io/ioutil"
)

// SettingEntity 設定ファイルのエンティティ
type SettingEntity struct {
	UniverseID       string                   `json:"UniverseID"`
	BrokerAddress    string                   `json:"BrokerAddress"`
	MinimumStepTime  int                      `json:"MinimumStepTime"`
	MapWidth         float64                  `json:"MapWidth"`
	MapHeight        float64                  `json:"MapHeight"`
	UseGPU           bool                     `json:"UseGPU"`
	FloodMeshSize    float64                  `json:"FloodMeshSize"`
	RootPath         string                   `json:"RootPath"`
	UniverseFilePath string                   `json:"UniverseFilePath"`
	FloodFilePath    string                   `json:"FloodFilePath"`
	Nodes            []SettingNodeEntity      `json:"Nodes"`
	Potentials       []SettingPotentialEntity `json:"Potential"`
}

type SettingNodeEntity struct {
	MaximumInfluenceLength int    `json:"MaximumInfluenceLength"`
	PersonFilePath         string `json:"PersonFilePath"`
	NodeFilePath           string `json:"NodeFilePath"`
	LinkFilePath           string `json:"LinkFilePath"`
	ShelterFilePath        string `json:"ShelterFilePath"`
}

type SettingPotentialEntity struct {
	MeshSize       float64                          `json:"MeshSize"`
	PersonFilePath string                           `json:"PersonFilePath"`
	InternalMaps   string                           `json:"InternalMaps"`
	ExternalMaps   []SettingPotentialExternalEntity `json:"ExternalMaps"`
	DisasterMaps   []SettingPotentialDisasterEntity `json:"DisasterMaps"`
	Media          []SettingPotentialMediaEntity    `json:"Media"`
}

type SettingPotentialExternalEntity struct {
	FilePath  string `json:"FilePath"`
	IsJSON    bool   `json:"IsJSON"`
	IsWall    bool   `json:"IsWall"`
	IsShelter bool   `json:"IsShelter"`
}

type SettingPotentialDisasterEntity struct {
	FilePath string `json:"FilePath"`
	Labels   []int  `json:"Labels"`
}

type SettingPotentialMediaEntity struct {
	Type        string                  `json:"Type"`
	Step        int                     `json:"Step"`
	Duration    int                     `json:"Duration"`
	Acquisition float64                 `json:"Acquisition"`
	Size        float64                 `json:"Size"`
	Positions   []SettingPositionEntity `json:"Positions"`
}

type SettingPositionEntity struct {
	X float64 `json:"X"`
	Y float64 `json:"Y"`
}

// LoadSettings 設定ファイルを読み込む
func LoadSettings(fileName string) SettingEntity {

	// ファイルを読み込む
	// buffer, _ := ioutil.ReadFile("../../../data/settings_dummy.json")
	// buffer, _ := ioutil.ReadFile("../../../data/settings.json")
	buffer, _ := ioutil.ReadFile(fileName)

	// JSONファイルを解釈
	var entity SettingEntity
	json.Unmarshal(buffer, &entity)
	return entity
}
