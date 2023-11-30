package aria_utility_nodes

import (
	"aria_utility_settings"
	"encoding/csv"
	"io"
	"math"
	"os"
	"strconv"
)

// NodeEntity 近隣のノード情報を含むノードのエンティティ
type NodeEntity struct {
	NID       int
	X         float64
	Y         float64
	Height    float64
	IsShelter bool
	Neighbors []NeighborEntity
	From      int
	Flood     float64
}

// NeighborEntity 近隣のノード＋そこまでの距離
type NeighborEntity struct {
	Node   *NodeEntity
	Length float64
}

// TODO : 最終的にマップサイズを設定ファイルから取得するように変更する
// LoadMap 地図情報（ノードとリンクを含む）を読み込む
func LoadMap(settings aria_utility_settings.SettingEntity, nodeEntity aria_utility_settings.SettingNodeEntity) map[int]*NodeEntity {
	var nodes map[int]*NodeEntity = make(map[int]*NodeEntity)

	// ノードCSVファイルの読込－－－－－－－－－－－－－－－－－－－－－－－－－－－－－－－－－－－－－－－－
	file, _ := os.Open(nodeEntity.NodeFilePath)
	reader := csv.NewReader(file)
	reader.FieldsPerRecord = -1

	// 解析
	reader.Read()
	reader.Read()
	reader.Read()
	reader.Read()
	reader.Read()
	for {
		line, e := reader.Read()
		if e == io.EOF {
			break
		}

		nid, _ := strconv.ParseInt(line[0], 10, 64)
		x, _ := strconv.ParseFloat(line[1], 64)
		y, _ := strconv.ParseFloat(line[2], 64)
		height, _ := strconv.ParseFloat(line[3], 64)

		nodes[int(nid)] = &NodeEntity{
			NID:       int(nid),
			X:         x,
			Y:         y,
			Height:    height,
			IsShelter: false,
			Neighbors: []NeighborEntity{},
			From:      -1,
			Flood:     0,
		}
	}
	// －－－－－－－－－－－－－－－－－－－－－－－－－－－－－－－－－－－－－－－－

	// リンクCSVファイルの読込－－－－－－－－－－－－－－－－－－－－－－－－－－－－－－－－－－－－－－－－
	file, _ = os.Open(nodeEntity.LinkFilePath)
	reader = csv.NewReader(file)
	reader.FieldsPerRecord = -1

	// 解析
	reader.Read()
	for {
		line, e := reader.Read()
		if e == io.EOF {
			break
		}

		nid1, _ := strconv.ParseInt(line[1], 10, 64)
		nid2, _ := strconv.ParseInt(line[2], 10, 64)
		length, _ := strconv.ParseFloat(line[3], 64)

		nodes[int(nid1)].Neighbors = append(nodes[int(nid1)].Neighbors, NeighborEntity{
			Node:   nodes[int(nid2)],
			Length: length,
		})
		nodes[int(nid2)].Neighbors = append(nodes[int(nid2)].Neighbors, NeighborEntity{
			Node:   nodes[int(nid1)],
			Length: length,
		})
	}
	// －－－－－－－－－－－－－－－－－－－－－－－－－－－－－－－－－－－－－－－－

	// シェルターCSVファイルの読み込み－－－－－－－－－－－－－－－－－－－－－－－－－－－－－－－－－－－－－－－－
	file, _ = os.Open(nodeEntity.ShelterFilePath)
	reader = csv.NewReader(file)
	reader.FieldsPerRecord = -1

	// 解析
	reader.Read()
	for {
		line, e := reader.Read()
		if e == io.EOF {
			break
		}

		x, _ := strconv.ParseFloat(line[1], 64)
		y, _ := strconv.ParseFloat(line[2], 64)
		x *= settings.FloodMeshSize
		y *= settings.FloodMeshSize

		max := math.MaxFloat64
		var target *NodeEntity
		for _, node := range nodes {
			length := (x-node.X)*(x-node.X) + (y-node.Y)*(y-node.Y)
			if length < max {
				max = length
				target = node
			}
		}
		target.IsShelter = true
	}
	// －－－－－－－－－－－－－－－－－－－－－－－－－－－－－－－－－－－－－－－－

	return nodes
}
