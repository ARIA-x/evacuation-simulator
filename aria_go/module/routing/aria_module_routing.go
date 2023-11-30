package aria_module_routing

import (
	"encoding/json"
	"fmt"
	"math"
	"strconv"
	"strings"
	"sync"

	"aria_utility_floods"
	"aria_utility_mqtt"
	"aria_utility_nodes"
	"aria_utility_settings"

	MQTT "github.com/eclipse/paho.mqtt.golang"
	"github.com/rs/xid"
)

type RoutingModule struct {
	client MQTT.Client
}

// Positionモジュール
type Position struct {
	X float64
	Y float64
}

func (routing *RoutingModule) Initialize(settings aria_utility_settings.SettingEntity, nodeEntity aria_utility_settings.SettingNodeEntity) *sync.WaitGroup {
	syncer := sync.WaitGroup{}
	syncer.Add(1)

	// マップファイルの読み込み
	nodes := aria_utility_nodes.LoadMap(settings, nodeEntity)
	mapWidth := settings.MapWidth
	mapHeight := settings.MapHeight
	floodWidth := int(math.Ceil(mapWidth / settings.FloodMeshSize))
	floodHeight := int(math.Ceil(mapHeight / settings.FloodMeshSize))

	// QR洪水情報の配列
	var qrFloods map[string]Position = make(map[string]Position)

	// ステップの開始
	var countRecieved MQTT.MessageHandler = func(client MQTT.Client, msg MQTT.Message) {
		var entity aria_utility_mqtt.CountEntity
		json.Unmarshal(msg.Payload(), &entity)

		// 洪水情報の処理
		floods, _, _ := aria_utility_floods.LoadFloods(settings, floodWidth, floodHeight, entity.Count)

		// QR洪水の追加
		for _, qrFlood := range qrFloods {
			px := int(qrFlood.X / settings.FloodMeshSize)
			py := int(qrFlood.Y / settings.FloodMeshSize)
			if px >= 0 && px < floodWidth && py >= 0 && py < floodHeight {
				floods[px][py] = 1.0
			}
		}

		for _, node := range nodes {
			node.From = -1
			node.Flood = floods[int(node.X/settings.FloodMeshSize)][int(node.Y/settings.FloodMeshSize)]
		}

		// 簡易経路計算
		tasks := []*aria_utility_nodes.NodeEntity{}
		for _, node := range nodes {
			if node.IsShelter {
				node.From = node.NID
				tasks = append(tasks, node)
			}
		}
		taskIndex := 0
		for {
			if taskIndex == len(tasks) {
				break
			}
			node := tasks[taskIndex]
			taskIndex++

			if node.Flood > 0.5 {
				continue
			}

			for _, neighbor := range node.Neighbors {
				if neighbor.Node.From < 0 {
					neighbor.Node.From = node.NID
					tasks = append(tasks, neighbor.Node)
				}
			}
		}

		// fmt.Printf("--- Route Updated %d ---\n", entity.Count)
	}

	// ルートリクエストの受信
	var routeRecieved MQTT.MessageHandler = func(client MQTT.Client, msg MQTT.Message) {
		var entity aria_utility_mqtt.RouteEntity
		json.Unmarshal(msg.Payload(), &entity)

		id := strings.Split(msg.Topic(), "/")[len(strings.Split(msg.Topic(), "/"))-1]

		route := []string{}
		routeNode := nodes[entity.StartNID]
		for {
			route = append(route, strconv.Itoa(routeNode.NID))
			if routeNode.IsShelter || routeNode.From == -1 {
				break
			}
			routeNode = nodes[routeNode.From]
		}

		if routeNode.IsShelter {
			bytes, _ := json.Marshal(route)
			token := client.Publish(fmt.Sprintf("/person/recv/start2target/%s", id), 0, false, bytes)
			token.Wait()

			// fmt.Printf("Routed %s (%d -> %d)\n", id, entity.StartNID, routeNode.NID)
		} else {
			// fmt.Printf("Routed %s (%d -> x)\n", id, entity.StartNID)
		}
	}

	var qrFloodRecieved MQTT.MessageHandler = func(client MQTT.Client, msg MQTT.Message) {
		var entity aria_utility_mqtt.CameraEntity
		json.Unmarshal(msg.Payload(), &entity)

		if qrFlood, exists := qrFloods[entity.Data]; exists {
			qrFlood.X = float64(entity.X-entity.Left) * mapWidth / float64(entity.Right-entity.Left)
			qrFlood.Y = float64(entity.Y-entity.Top) * mapHeight / float64(entity.Bottom-entity.Top)
		} else {
			qrFloods[entity.Data] = Position{
				X: float64(entity.X-entity.Left) * mapWidth / float64(entity.Right-entity.Left),
				Y: float64(entity.Y-entity.Top) * mapHeight / float64(entity.Bottom-entity.Top),
			}
		}
	}

	// MQTTクライアントの設定
	opts := MQTT.NewClientOptions().AddBroker(settings.BrokerAddress).SetClientID(xid.New().String())
	opts.OnConnect = func(client MQTT.Client) {
		if token := client.Subscribe("/person/send/start2target/+", 0, routeRecieved); token.Wait() && token.Error() != nil {
			panic(token.Error())
		}
		if token := client.Subscribe("/flood/count", 0, countRecieved); token.Wait() && token.Error() != nil {
			panic(token.Error())
		}
		if token := client.Subscribe("/camera/flood/+", 0, qrFloodRecieved); token.Wait() && token.Error() != nil {
			panic(token.Error())
		}
		fmt.Printf("[Routing ] Initialized (%s)\n", opts.ClientID)
		syncer.Done()
	}

	// MQTTブローカーに接続
	routing.client = MQTT.NewClient(opts)
	if token := routing.client.Connect(); token.Wait() && token.Error() != nil {
		panic(token.Error())
	}

	return &syncer
}

func (routing *RoutingModule) Uninitialize() {
	routing.client.Disconnect(250)
	fmt.Println("[Routing ] Uninitialize")
}
