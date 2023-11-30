package aria_module_media

import (
	"encoding/json"
	"fmt"
	"sync"

	"aria_utility_mqtt"
	"aria_utility_settings"

	MQTT "github.com/eclipse/paho.mqtt.golang"
	"github.com/rs/xid"
)

// Mediaモジュール
type MediaModule struct {
	client MQTT.Client
}

func (module *MediaModule) Initialize(settings aria_utility_settings.SettingEntity, potentialEntity aria_utility_settings.SettingPotentialEntity) *sync.WaitGroup {
	syncer := sync.WaitGroup{}
	syncer.Add(1)

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

		for _, mediaEntity := range potentialEntity.Media {
			currentStep := entity.Count - mediaEntity.Step
			if 0 <= currentStep && currentStep <= mediaEntity.Duration {
				entity := aria_utility_mqtt.MediaEntity{
					X:           mediaEntity.Positions[currentStep%len(mediaEntity.Positions)].X,
					Y:           mediaEntity.Positions[currentStep%len(mediaEntity.Positions)].Y,
					Size:        mediaEntity.Size,
					Acquisition: mediaEntity.Acquisition,
					Type:        mediaEntity.Type,
				}
				bytes, _ := json.Marshal(entity)
				if token := client.Publish(fmt.Sprintf("aria/media/%s", settings.UniverseID), 0, false, bytes); token.Wait() && token.Error() != nil {
					panic(token.Error())
				}
			}
		}
	}

	// MQTTクライアントの設定
	opts := MQTT.NewClientOptions().AddBroker(settings.BrokerAddress).SetClientID(xid.New().String())
	opts.OnConnect = func(client MQTT.Client) {
		if token := client.Subscribe("/flood/count", 0, countRecieved); token.Wait() && token.Error() != nil {
			panic(token.Error())
		}
		fmt.Printf("[Media] Initialized (%s)\n", opts.ClientID)
		syncer.Done()
	}

	// MQTTブローカーに接続
	module.client = MQTT.NewClient(opts)
	if token := module.client.Connect(); token.Wait() && token.Error() != nil {
		panic(token.Error())
	}

	return &syncer
}

func (person *MediaModule) Uninitialize() {
	person.client.Disconnect(250)
	fmt.Println("[Media] Uninitialize")
}
