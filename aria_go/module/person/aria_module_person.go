package aria_module_person

import (
	"aria_utility_settings"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"os"
	"sort"
	"strconv"
	"strings"
	"sync"

	"aria_utility_floods"
	"aria_utility_mqtt"
	"aria_utility_nodes"

	MQTT "github.com/eclipse/paho.mqtt.golang"
	"github.com/rs/xid"
)

// Personのファイルデータ
type PersonData struct {
	NID            int
	X              float64
	Y              float64
	InfoAccess     int
	PrepareTimeout int
	Speed          float64
	ViewLength     int
	WarningDepth   float64
	VictimDepth    float64
	TargetNID      int
	RequestTimeout int
	RerouteTimeout int
	Influence      int
}

// Personエージェント
type Person struct {
	NID            int
	X              float64
	Y              float64
	WayToNode      float64
	Status         int
	InfoAccess     int
	Route          []int
	PrepareTimeout int
	RerouteTimeout int
	Data           PersonData
	IsAnnounced    bool
	RouteToLeader  []int
	RouteToTop     []int
}

// 他のPersonモジュールも含めたPerson
type PersonTemporary struct {
	ID        int
	NID       int   `json:"nid"`
	Influence int   `json:"influence"`
	Route     []int `json:"route"`
}

// Positionエージェント
type Position struct {
	X float64
	Y float64
}

// ViewPointエージェント
type ViewPoint struct {
	X      int
	Y      int
	Length int
}

type PersonModule struct {
	client      MQTT.Client
	Nodes       map[int]*aria_utility_nodes.NodeEntity
	MapWidth    float64
	MapHeight   float64
	Floods      [][]float64
	FloodWidth  int
	FloodHeight int
}

func (module *PersonModule) Initialize(settings aria_utility_settings.SettingEntity, nodeEntity aria_utility_settings.SettingNodeEntity) *sync.WaitGroup {
	syncer := sync.WaitGroup{}
	syncer.Add(1)

	// マップファイルの読み込み
	module.Nodes = aria_utility_nodes.LoadMap(settings, nodeEntity)
	module.MapWidth = settings.MapWidth
	module.MapHeight = settings.MapHeight
	module.FloodWidth = int(math.Ceil(module.MapWidth / settings.FloodMeshSize))
	module.FloodHeight = int(math.Ceil(module.MapHeight / settings.FloodMeshSize))

	// ノードの数と近隣ノードの数をカウントしておく（メモリ確保用）
	nodeIDMax := 0
	totalNeighbor := 0
	for _, node := range module.Nodes {
		if nodeIDMax < node.NID {
			nodeIDMax = node.NID
		}
		totalNeighbor += len(node.Neighbors)
	}

	// 内部設定値
	moduleID := ""
	personIDFrom := 0
	personIDTo := 0
	announceStep := 0

	// 初期のパーソンの配列
	personDatas := []PersonData{}

	// パーソンの配列
	var persons map[int]*Person

	// 他のモジュールも含めたパーソンの配列
	personsInUniverse := make(map[int]PersonTemporary)

	// QR洪水の座標一覧
	var qrFloods map[string]Position = make(map[string]Position)

	// QRアンテナの座標一覧
	var qrAntennas map[string]Position = make(map[string]Position)

	// 視野データの生成
	viewPoints := []ViewPoint{}
	for x := -10; x <= 10; x++ {
		for y := -10; y <= 10; y++ {
			length := int(math.Ceil(math.Sqrt(float64(x*x + y*y))))
			if length <= 10 {
				viewPoints = append(viewPoints, ViewPoint{
					X:      x,
					Y:      y,
					Length: length,
				})
			}
		}
	}
	sort.Slice(viewPoints, func(i, j int) bool { return viewPoints[i].Length < viewPoints[j].Length })

	// パーソン設定ファイルの読込－－－－－－－－－－－－－－－－－－－－－－－－－－－－－－－－－－－－－－－－

	// ファイルを開く
	file, _ := os.Open(nodeEntity.PersonFilePath)
	reader := csv.NewReader(file)
	reader.FieldsPerRecord = -1

	// 設定ファイルの内容を解析する
	reader.Read()
	for {
		line, e := reader.Read()
		if e == io.EOF {
			break
		}

		x, _ := strconv.ParseFloat(line[0], 64)
		y, _ := strconv.ParseFloat(line[1], 64)
		info, _ := strconv.Atoi(line[2])
		prep, _ := strconv.Atoi(line[3])
		speed, _ := strconv.ParseFloat(line[4], 64)
		view, _ := strconv.Atoi(line[7])
		warning, _ := strconv.ParseFloat(line[8], 64)
		victim, _ := strconv.ParseFloat(line[9], 64)
		target, _ := strconv.Atoi(line[10])
		request, _ := strconv.Atoi(line[11])
		reroute, _ := strconv.Atoi(line[12])
		influence, _ := strconv.Atoi(line[13])

		// 指定された座標から最も近い「ノード」にパーソンを配置する（実際の座標は無視している）
		max := math.MaxFloat64
		var current int
		for _, node := range module.Nodes {
			length := (x-node.X)*(x-node.X) + (y-node.Y)*(y-node.Y)
			if length < max {
				max = length
				current = node.NID
			}
		}

		personDatas = append(personDatas, PersonData{
			NID:            current,
			X:              module.Nodes[current].X,
			Y:              module.Nodes[current].Y,
			InfoAccess:     info,
			PrepareTimeout: prep,
			Speed:          speed,
			ViewLength:     view,
			WarningDepth:   warning,
			VictimDepth:    victim,
			TargetNID:      target,
			RequestTimeout: request,
			RerouteTimeout: reroute,
			Influence:      influence,
		})
	}
	// －－－－－－－－－－－－－－－－－－－－－－－－－－－－－－－－－－－－－－－－

	// パーソンエージェントの参加完了
	var registeredRecieved MQTT.MessageHandler = func(client MQTT.Client, msg MQTT.Message) {
		var entity aria_utility_mqtt.RegisteredEntity
		json.Unmarshal(msg.Payload(), &entity)

		personIDFrom = entity.From
		personIDTo = entity.To
		fmt.Printf("[Person  ] Registered : %s (%d to %d)\n", entity.ID, personIDFrom, personIDTo)
		syncer.Done()
	}

	// サイクルの開始
	var cycleRecieved MQTT.MessageHandler = func(client MQTT.Client, msg MQTT.Message) {
		var entity aria_utility_mqtt.CycleEntity
		json.Unmarshal(msg.Payload(), &entity)

		// 設定
		announceStep = entity.AnnounceStep

		// パーソンの新規作成
		persons = make(map[int]*Person)
		index := 0
		for i := personIDFrom; i < personIDTo; i++ {
			persons[i] = &Person{
				NID:            personDatas[index].NID,
				X:              personDatas[index].X,
				Y:              personDatas[index].Y,
				WayToNode:      0.0,
				Route:          []int{},
				Status:         1,
				InfoAccess:     personDatas[index].InfoAccess,
				PrepareTimeout: personDatas[index].PrepareTimeout,
				RerouteTimeout: 0,
				Data:           personDatas[index],
				IsAnnounced:    false,
			}

			// 最初から避難場所にいるパターン
			if module.Nodes[persons[i].NID].IsShelter {
				persons[i].Status = 7
			}
			index++
		}

		// 準備完了をPublish
		bytes, _ := json.Marshal(aria_utility_mqtt.PreparedEntity{
			ID: moduleID,
		})
		if token := client.Publish(fmt.Sprintf("aria/prepared/%s", settings.UniverseID), 0, false, bytes); token.Wait() && token.Error() != nil {
			panic(token.Error())
		}
	}

	// ステップの開始
	var countRecieved MQTT.MessageHandler = func(client MQTT.Client, msg MQTT.Message) {
		if !module.client.IsConnected() {
			return
		}

		var entity aria_utility_mqtt.CountEntity
		json.Unmarshal(msg.Payload(), &entity)
		if entity.Count < 0 {
			return
		}

		// 洪水情報の処理
		module.Floods, _, _ = aria_utility_floods.LoadFloods(settings, module.FloodWidth, module.FloodHeight, entity.Count)

		// QR洪水の追加
		for _, qrFlood := range qrFloods {
			px := int(qrFlood.X / settings.FloodMeshSize)
			py := int(qrFlood.Y / settings.FloodMeshSize)
			if px >= 0 && px < module.FloodWidth && py >= 0 && py < module.FloodHeight {
				module.Floods[px][py] = 1.0
			}
		}

		// アナウンス
		if entity.Count == announceStep {
			for _, person := range persons {
				person.IsAnnounced = true
			}
		}

		// パーソンをマッピング
		nodeBuffer := make([]int32, (nodeIDMax+1)*7)
		neighborBuffer := make([]int32, totalNeighbor)
		currentNeighbor := int32(0)
		for id, node := range module.Nodes {

			x := int(node.X / settings.FloodMeshSize)
			y := int(node.Y / settings.FloodMeshSize)
			if module.Floods[x][y] > 0.5 {
				continue
			}

			nodeBuffer[id*7+0] = int32(node.X)              // X座標
			nodeBuffer[id*7+1] = int32(node.Y)              // Y座標
			nodeBuffer[id*7+2] = int32(node.Height)         // 高さ
			nodeBuffer[id*7+3] = -1                         // 最大影響力パーソンのID
			nodeBuffer[id*7+4] = -1                         // 最大影響力パーソンの影響力
			nodeBuffer[id*7+5] = currentNeighbor            // 近接ノードの開始Index
			nodeBuffer[id*7+6] = int32(len(node.Neighbors)) // 近接ノードの数
			for _, neighbor := range node.Neighbors {
				neighborBuffer[currentNeighbor] = int32(neighbor.Node.NID)
				currentNeighbor++
			}
		}
		for id, person := range personsInUniverse {
			if nodeBuffer[person.NID*7+4] < int32(person.Influence) {
				nodeBuffer[person.NID*7+3] = int32(id)
				nodeBuffer[person.NID*7+4] = int32(person.Influence)
			}
		}

		// 事前に相互作用の計算
		tasks := make([]int32, len(personDatas)*1000)
		froms := make([]int32, len(personDatas)*1000)
		personBuffer := make([]int32, len(personDatas)*7)
		personIndex := int32(0)
		for id, person := range persons {
			personBuffer[personIndex*7+0] = int32(id)
			personBuffer[personIndex*7+1] = int32(person.Status)
			personBuffer[personIndex*7+2] = int32(person.NID)
			personBuffer[personIndex*7+3] = int32(person.X)
			personBuffer[personIndex*7+4] = int32(person.Y)
			personBuffer[personIndex*7+5] = 0
			personBuffer[personIndex*7+6] = 0
			personIndex++
		}

		// goroutineの処理
		signature := make(chan int, 100)
		defer close(signature)
		responce := make(chan int, len(personDatas))
		defer close(responce)
		for personIndex = 0; personIndex < int32(len(personDatas)); personIndex++ {
			go search(signature, responce, personIndex, tasks, froms, personBuffer, nodeBuffer, neighborBuffer, int32(nodeEntity.MaximumInfluenceLength))
		}
		for {
			//全部が終わるまで待つ
			if len(responce) == len(personDatas) {
				break
			}
		}

		// ルートの後処理
		for personIndex = 0; personIndex < int32(len(personDatas)); personIndex++ {
			person := persons[int(personBuffer[personIndex*7+0])]

			// もっとも影響力が高いパーソンへのルートを追加
			person.RouteToLeader = person.RouteToLeader[:0]
			for froms[personIndex*1000+personBuffer[personIndex*7+6]] != -1 {
				person.RouteToLeader = append([]int{int(tasks[personIndex*1000+personBuffer[personIndex*7+6]])}, person.RouteToLeader...)
				personBuffer[personIndex*7+6] = froms[personIndex*1000+personBuffer[personIndex*7+6]]
			}

			// もっとも標高が高いノードへのルートを追加
			person.RouteToTop = person.RouteToTop[:0]
			for froms[personIndex*1000+personBuffer[personIndex*7+5]] != -1 {
				person.RouteToTop = append([]int{int(tasks[personIndex*1000+personBuffer[personIndex*7+5]])}, person.RouteToTop...)
				personBuffer[personIndex*7+5] = froms[personIndex*1000+personBuffer[personIndex*7+5]]
			}
		}

		// 各パーソンの処理を実行
		for id, person := range persons {

			// 被災済み、または避難済みは、外部からの影響も受けない
			if person.Status == 6 || person.Status == 7 {
				continue
			}

			// 被災（外部からの影響）
			x := int(person.X / settings.FloodMeshSize)
			y := int(person.Y / settings.FloodMeshSize)
			if module.Floods[x][y]-module.Nodes[person.NID].Height/100.0 >= person.Data.VictimDepth {
				person.Status = 6
				continue
			}

			// 近くのQRアンテナを探す（外部からの影響）
			for _, qrAntenna := range qrAntennas {
				length := (qrAntenna.X-person.X)*(qrAntenna.X-person.X) + (qrAntenna.Y-person.Y)*(qrAntenna.Y-person.Y)
				if length <= 1000000 {
					person.InfoAccess = 0
				}
			}

			if person.Data.Influence != 0 && len(person.RouteToLeader) > 0 {
				influence := int(nodeBuffer[person.RouteToLeader[len(person.RouteToLeader)-1]*7+4])
				if influence > person.Data.Influence {
					if influence >= 2 {
						person.IsAnnounced = true
					}
					if influence >= 3 {
						person.PrepareTimeout = 0
					}
					if person.Status != 5 && (len(person.Route) == 0 || influence == 4) {
						target := personsInUniverse[int(nodeBuffer[person.RouteToLeader[len(person.RouteToLeader)-1]*7+3])]
						person.Route = person.RouteToLeader
						person.Route = append(person.Route, target.Route...)
						if person.PrepareTimeout <= 0 {
							person.Status = 5
						}
					}
				}
			}

			// 警報前はルートリクエストも移動もしない
			if !person.IsAnnounced {
				continue
			}

			// 避難準備中はルートリクエストも移動もしない
			person.PrepareTimeout--
			person.RerouteTimeout--
			if person.PrepareTimeout > 0 {
				continue
			}

			// ルートリクエスト
			isReRequesting := false
			if person.RerouteTimeout <= 0 && person.InfoAccess == 1 {

				// ルートを持っていない、あるいは視界内に洪水があるかどうか調べる
				reroute := false
				if len(person.Route) == 0 {
					reroute = true
				} else {
					for _, viewPoint := range viewPoints {
						if viewPoint.Length > person.Data.ViewLength {
							break
						}
						px := x + viewPoint.X
						py := y + viewPoint.Y
						if px >= 0 && px < module.FloodWidth && py >= 0 && py < module.FloodHeight && module.Floods[px][py] >= person.Data.WarningDepth {
							reroute = true
						}
					}
				}

				// 対象であればルートリクエストを実行
				if reroute {
					if person.Status == 2 {
						isReRequesting = true
					}
					person.Status = 2
					person.Route = person.Route[:0]
					person.RerouteTimeout = person.Data.RequestTimeout

					// 経路要求をPublish
					bytes, _ := json.Marshal(aria_utility_mqtt.RouteEntity{
						StartNID:  person.NID,
						TargetNID: person.Data.TargetNID,
					})
					if token := client.Publish(fmt.Sprintf("/person/send/start2target/%d", id), 0, false, bytes); token.Wait() && token.Error() != nil {
						panic(token.Error())
					}
				}
			}

			// 高所避難中、2回目の通信待機中、
			if len(person.RouteToTop) > 0 && (person.Status == 4 || isReRequesting || (person.Status <= 2 && person.InfoAccess == 0)) {
				person.Status = 4
				person.Route = person.RouteToTop
			}

			// 移動
			if len(person.Route) > 0 {
				remainingLength := person.Data.Speed
				for remainingLength > 0 {
					currentNode := module.Nodes[person.NID]
					targetNode := module.Nodes[person.Route[0]]
					nodeToNode := -1.0

					// 次のノードが近隣にノードにあるか調べる
					for _, neighbor := range currentNode.Neighbors {
						if neighbor.Node.NID == person.Route[0] {
							nodeToNode = neighbor.Length
						}
					}
					if nodeToNode == -1 {
						panic("Target Node Is Not Neighbor")
					}

					if person.WayToNode+remainingLength/nodeToNode >= 1.0 {

						// 次のノードまで移動
						person.NID = person.Route[0]
						person.X = targetNode.X
						person.Y = targetNode.Y
						person.Route = person.Route[1:]
						remainingLength -= nodeToNode * (1.0 - person.WayToNode)
						person.WayToNode = 0

						// ゴール
						if len(person.Route) == 0 {
							if module.Nodes[person.NID].IsShelter {
								person.Status = 7
							} else {
								person.Status = 1
							}
							break
						}
					} else {

						// ノードの途中まで移動
						person.WayToNode = person.WayToNode + remainingLength/nodeToNode
						person.X = targetNode.X*person.WayToNode + currentNode.X*(1.0-person.WayToNode)
						person.Y = targetNode.Y*person.WayToNode + currentNode.Y*(1.0-person.WayToNode)
						remainingLength = 0
					}
				}
			}
		}

		// 結果をPublish
		results := []aria_utility_mqtt.AllEntity{}
		for id, person := range persons {
			results = append(results, aria_utility_mqtt.AllEntity{
				Count:      entity.Count,
				ID:         id,
				X:          person.X,
				Y:          person.Y,
				Status:     person.Status,
				InfoAccess: person.InfoAccess,
			})
		}
		bytes, _ := json.Marshal(aria_utility_mqtt.StepEntity{
			ID:      moduleID,
			Persons: results,
		})
		if token := client.Publish(fmt.Sprintf("aria/persons/%s", settings.UniverseID), 0, false, bytes); token.Wait() && token.Error() != nil {
			panic(token.Error())
		}

		// TODO : 最終的にはどうにかしてAllと共通化するべき
		// 結果を内部にPublish
		results2 := []PersonTemporary{}
		for id, person := range persons {
			results2 = append(results2, PersonTemporary{
				ID:        id,
				NID:       person.NID,
				Influence: person.Data.Influence,
				Route:     person.Route,
				// Z:         module.Nodes[person.NID].Height,
			})
		}
		bytes2, _ := json.Marshal(results2)
		if token := client.Publish(fmt.Sprintf("aria/intra/persons/%s", settings.UniverseID), 0, false, bytes2); token.Wait() && token.Error() != nil {
			panic(token.Error())
		}
	}

	// ルーティングの完了
	var routedRecieved MQTT.MessageHandler = func(client MQTT.Client, msg MQTT.Message) {
		var entity []string
		json.Unmarshal(msg.Payload(), &entity)

		// PersonのIDを解析
		id, _ := strconv.Atoi(strings.Split(msg.Topic(), "/")[len(strings.Split(msg.Topic(), "/"))-1])

		// Personが存在する場合は処理
		if person, exists := persons[id]; exists {

			// 受信済みであれば無視する
			if len(person.Route) == 0 {

				// ルートを追加
				for _, node := range entity {
					nid, _ := strconv.Atoi(node)
					person.Route = append(person.Route, nid)
				}

				// 起点がずれた受信は無視する
				if len(person.Route) > 0 && person.NID != person.Route[0] {
					person.Route = person.Route[:0]
				} else {
					person.Status = 3
					person.Route = person.Route[1:]
					person.RerouteTimeout = person.Data.RerouteTimeout
				}
			}
		}
	}

	// QR洪水の情報を受信
	var qrFloodRecieved MQTT.MessageHandler = func(client MQTT.Client, msg MQTT.Message) {
		var entity aria_utility_mqtt.CameraEntity
		json.Unmarshal(msg.Payload(), &entity)

		x := float64(entity.X-entity.Left) * module.MapWidth / float64(entity.Right-entity.Left)
		y := float64(entity.Y-entity.Top) * module.MapHeight / float64(entity.Bottom-entity.Top)
		if qrFlood, exists := qrFloods[entity.Data]; exists {
			qrFlood.X = x
			qrFlood.Y = y
		} else {
			qrFloods[entity.Data] = Position{
				X: x,
				Y: y,
			}
		}
	}

	// QRアンテナの情報を受信
	var qrAntennaRecieved MQTT.MessageHandler = func(client MQTT.Client, msg MQTT.Message) {
		var entity aria_utility_mqtt.CameraEntity
		json.Unmarshal(msg.Payload(), &entity)

		x := float64(entity.X-entity.Left) * module.MapWidth / float64(entity.Right-entity.Left)
		y := float64(entity.Y-entity.Top) * module.MapHeight / float64(entity.Bottom-entity.Top)
		if qrAntenna, exists := qrAntennas[entity.Data]; exists {
			qrAntenna.X = x
			qrAntenna.Y = y
		} else {
			qrAntennas[entity.Data] = Position{
				X: x,
				Y: y,
			}
		}
	}

	// メッセージを受信
	var messageRecieved MQTT.MessageHandler = func(client MQTT.Client, msg MQTT.Message) {
		var entity aria_utility_mqtt.MessageEntity
		json.Unmarshal(msg.Payload(), &entity)

		for _, target := range entity.Persons {
			if _, ok := persons[target.ID]; ok {
				print("Person ")
				print(target.ID)
				print(" Message Recieved")
				println()
			}
		}

		for _, target := range entity.Nodes {
			for id, person := range persons {
				if person.NID == target.ID {
					print("Person ")
					print(id)
					print(" Message Recieved")
					println()
				}
			}
		}

		for _, target := range entity.Areas {
			for id, person := range persons {
				if (target.X-person.X)*(target.X-person.X)+(target.Y-person.Y)*(target.Y-person.Y) <= target.Size*target.Size {
					print("Person ")
					print(id)
					print(" Message Recieved")
					println()
				}
			}
		}
	}

	// パーソンエージェントのステップ完了メッセージを受信
	var intraRecieved MQTT.MessageHandler = func(client MQTT.Client, msg MQTT.Message) {
		var entity []PersonTemporary
		json.Unmarshal(msg.Payload(), &entity)

		// パーソンエージェントを追加
		for _, person := range entity {
			personsInUniverse[person.ID] = person
		}

		// TODO : 確実に待たせる実装が必要かも
	}

	// MQTTクライアントの設定
	opts := MQTT.NewClientOptions().AddBroker(settings.BrokerAddress).SetClientID(xid.New().String())
	opts.OnConnect = func(client MQTT.Client) {

		// モジュールIDをランダムに設定
		moduleID = xid.New().String()

		// MQTTのサブスクライブ
		if token := client.Subscribe(fmt.Sprintf("aria/registered/%s/%s", settings.UniverseID, moduleID), 0, registeredRecieved); token.Wait() && token.Error() != nil {
			panic(token.Error())
		}
		if token := client.Subscribe(fmt.Sprintf("aria/cycle/%s", settings.UniverseID), 0, cycleRecieved); token.Wait() && token.Error() != nil {
			panic(token.Error())
		}
		if token := client.Subscribe("/flood/count", 0, countRecieved); token.Wait() && token.Error() != nil {
			panic(token.Error())
		}
		if token := client.Subscribe("/person/recv/start2target/+", 0, routedRecieved); token.Wait() && token.Error() != nil {
			panic(token.Error())
		}
		if token := client.Subscribe("/camera/flood/+", 0, qrFloodRecieved); token.Wait() && token.Error() != nil {
			panic(token.Error())
		}
		if token := client.Subscribe("/camera/antenna/+", 0, qrAntennaRecieved); token.Wait() && token.Error() != nil {
			panic(token.Error())
		}
		if token := client.Subscribe(fmt.Sprintf("aria/message/%s", settings.UniverseID), 0, messageRecieved); token.Wait() && token.Error() != nil {
			panic(token.Error())
		}
		if token := client.Subscribe(fmt.Sprintf("aria/intra/persons/%s", settings.UniverseID), 0, intraRecieved); token.Wait() && token.Error() != nil {
			panic(token.Error())
		}

		// Universeモジュールに参加をPublish
		bytes, _ := json.Marshal(aria_utility_mqtt.AttendEntity{
			ID:    moduleID,
			Count: len(personDatas),
		})
		if token := client.Publish(fmt.Sprintf("aria/attend/%s", settings.UniverseID), 0, false, bytes); token.Wait() && token.Error() != nil {
			panic(token.Error())
		}

		fmt.Printf("[Person  ] Initialized (%s)\n", opts.ClientID)
	}

	// MQTTブローカーに接続
	module.client = MQTT.NewClient(opts)
	if token := module.client.Connect(); token.Wait() && token.Error() != nil {
		panic(token.Error())
	}

	return &syncer
}

func (person *PersonModule) Uninitialize() {
	person.client.Disconnect(250)
	fmt.Println("[Person  ] Uninitialize")
}

// GPUがない場合の相互作用検索
func search(signature chan int, responce chan int, index int32, tasks []int32, froms []int32, personBuffer []int32, nodeBuffer []int32, neighborBuffer []int32, influrenceLength int32) {
	signature <- 0

	status := personBuffer[index*7+1]
	nodeID := personBuffer[index*7+2]
	personX := personBuffer[index*7+3]
	personY := personBuffer[index*7+4]
	tasks[index*1000+0] = nodeID
	froms[index*1000+0] = -1

	// 被災済み、または避難済みは、外部からの影響も受けない
	if status != 6 && status != 7 {
		taskIndex := int32(0)
		taskCount := int32(1)
		topPersonValue := int32(0)
		topNodeValue := nodeBuffer[nodeID*7+2]

		// 周囲のパーソン検索（外部からの影響）
		for {
			// タスクを全てチェックし終わったら終了
			if taskIndex >= taskCount {
				break
			}

			// チェック対象のノードを取得
			nodeID := tasks[index*1000+taskIndex]
			taskIndex++

			// 一番高いところを記録しておく
			if topNodeValue < nodeBuffer[nodeID*7+2] {
				personBuffer[index*7+5] = taskIndex - 1
				topNodeValue = nodeBuffer[nodeID*7+2]
			}

			// 一番影響力の高いパーソンを記録しておく
			if topPersonValue < nodeBuffer[nodeID*7+4] {
				personBuffer[index*7+6] = taskIndex - 1
				topPersonValue = nodeBuffer[nodeID*7+4]
			}

			// 近所のノードをタスクに追加する
			for i := int32(0); i < nodeBuffer[nodeID*7+6]; i++ {

				if taskCount == 1000 {
					println("Task Overflow")
					break
				}
				neighborNodeID := neighborBuffer[nodeBuffer[nodeID*7+5]+i]

				neighborNodeX := nodeBuffer[neighborNodeID*7+0]
				neighborNodeY := nodeBuffer[neighborNodeID*7+1]
				if (neighborNodeX-personX)*(neighborNodeX-personX)+(neighborNodeY-personY)*(neighborNodeY-personY) > influrenceLength*influrenceLength {
					continue
				}

				// チェック済みチェック
				exists := false
				for j := int32(0); j < taskCount; j++ {
					if tasks[index*1000+j] == neighborNodeID {
						exists = true
					}
				}
				if exists {
					continue
				}

				tasks[index*1000+taskCount] = neighborNodeID
				froms[index*1000+taskCount] = taskIndex - 1
				taskCount++
			}
		}
	}

	responce <- 0

	<-signature
}
