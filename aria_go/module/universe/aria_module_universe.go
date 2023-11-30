package aria_module_universe

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strconv"
	"sync"
	"time"

	"aria_utility_floods"
	"aria_utility_mqtt"
	"aria_utility_settings"

	MQTT "github.com/eclipse/paho.mqtt.golang"
	"github.com/rs/xid"
)

// Cycleエージェント
type Cycle struct {
	AnnounceStep int
	StepCount    int
}

// Personモジュール
type PersonModule struct {
	IsFinished bool
}

type UniverseModule struct {
	CycleCount      int
	StepCount       int
	lastStep        time.Time
	personModules   map[string]*PersonModule // 登録済みのPersonモジュール
	cycles          []Cycle                  // 設定ファイルのCycle情報一覧
	settings        aria_utility_settings.SettingEntity
	client          MQTT.Client
	Persons         []aria_utility_mqtt.AllEntity // 集計済みのPersonエージェント
	syncer          sync.WaitGroup
	NeedsCycleStart bool
	Affected        int
	Evacuated       int
}

func (universe *UniverseModule) Initialize(settings aria_utility_settings.SettingEntity) *sync.WaitGroup {
	syncer := sync.WaitGroup{}
	syncer.Add(1)

	universe.CycleCount = 0
	universe.NeedsCycleStart = true

	// 共通設定ファイルの読み込み
	universe.settings = settings

	// 登録済みのPersonモジュール
	universe.personModules = make(map[string]*PersonModule)

	// 設定ファイルの読み込み－－－－－－－－－－－－－－－－－－－－－－－－－－－－－－－－－－－－－－－－
	file, _ := os.Open(settings.UniverseFilePath)
	reader := csv.NewReader(file)
	reader.FieldsPerRecord = -1
	reader.Read()
	for {
		line, e := reader.Read()
		if e == io.EOF || len(line) < 2 {
			break
		}

		announceStep, _ := strconv.Atoi(line[0])
		stepCount, _ := strconv.Atoi(line[1])

		universe.cycles = append(universe.cycles, Cycle{
			AnnounceStep: announceStep,
			StepCount:    stepCount,
		})
	}
	// －－－－－－－－－－－－－－－－－－－－－－－－－－－－－－－－－－－－－－－－

	// パーソンエージェントの参加メッセージを受信
	personCount := 0
	var attendRecieved MQTT.MessageHandler = func(client MQTT.Client, msg MQTT.Message) {
		var entity aria_utility_mqtt.AttendEntity
		json.Unmarshal(msg.Payload(), &entity)

		// モジュールを登録
		universe.personModules[entity.ID] = &PersonModule{
			IsFinished: false,
		}

		// メッセージ出力
		print("[Universe] Person Modules : [")
		for id, personModule := range universe.personModules {
			print(id, " (", personModule.IsFinished, "), ")
		}
		println("]")

		// 登録されたことをPublish
		bytes, _ := json.Marshal(aria_utility_mqtt.RegisteredEntity{
			ID:   entity.ID,
			From: personCount,
			To:   personCount + entity.Count,
		})
		token := client.Publish(fmt.Sprintf("aria/registered/%s/%s", universe.settings.UniverseID, entity.ID), 0, false, bytes)
		token.Wait()

		personCount += entity.Count
	}

	// パーソンエージェントのサイクル準備完了メッセージを受信
	var preparedRecieved MQTT.MessageHandler = func(client MQTT.Client, msg MQTT.Message) {
		var entity aria_utility_mqtt.PreparedEntity
		json.Unmarshal(msg.Payload(), &entity)

		// このモジュールが完了したことを記録
		universe.personModules[entity.ID].IsFinished = true

		// パーソンエージェントを追加
		for _, person := range entity.Persons {
			universe.Persons = append(universe.Persons, aria_utility_mqtt.AllEntity{
				Count:      universe.StepCount,
				ID:         person.ID,
				X:          person.X / universe.settings.FloodMeshSize,
				Y:          person.Y / universe.settings.FloodMeshSize,
				Status:     person.Status,
				InfoAccess: person.InfoAccess,
			})
		}

		// 全てのモジュールが終わっていない場合は終了
		for _, personModule := range universe.personModules {
			if !personModule.IsFinished {
				return
			}
		}

		universe.NeedsCycleStart = false
		universe.syncer.Done()
	}

	// パーソンエージェントのステップ完了メッセージを受信
	var stepRecieved MQTT.MessageHandler = func(client MQTT.Client, msg MQTT.Message) {
		var entity aria_utility_mqtt.StepEntity
		json.Unmarshal(msg.Payload(), &entity)

		// このモジュールが完了したことを記録
		universe.personModules[entity.ID].IsFinished = true

		// パーソンエージェントを追加
		for _, person := range entity.Persons {
			universe.Persons = append(universe.Persons, aria_utility_mqtt.AllEntity{
				Count:      universe.StepCount,
				ID:         person.ID,
				X:          person.X / universe.settings.FloodMeshSize,
				Y:          person.Y / universe.settings.FloodMeshSize,
				Status:     person.Status,
				InfoAccess: person.InfoAccess,
			})
		}

		// 全てのパーソンモジュールが終わっていない場合は終了
		for _, personModule := range universe.personModules {
			if !personModule.IsFinished {
				return
			}
		}

		//allをPublish
		bytes, _ := json.Marshal(universe.Persons)
		token := client.Publish("/person/send/all", 0, false, bytes)
		token.Wait()

		// 洪水情報の処理（洪水情報をここで管理する必要は本当は無い）
		_, total, max := aria_utility_floods.LoadFloods(universe.settings, 0, 0, universe.StepCount)

		// 被災状況を計算
		universe.Affected = 0
		universe.Evacuated = 0
		for _, person := range universe.Persons {
			if person.Status == 6 {
				universe.Affected++
			}
			if person.Status == 7 {
				universe.Evacuated++
			}
		}

		// statをPublish
		bytes, _ = json.Marshal(aria_utility_mqtt.StatusEntity{
			AffectedPerson:  universe.Affected,
			EvacuatedPerson: universe.Evacuated,
			TotalFlood:      total,
			MaxFlood:        max,
		})
		token = client.Publish("/stat/send", 0, false, bytes)
		token.Wait()
		// fmt.Printf("Affected : %d\n", universe.Affected)
		// fmt.Printf("Evacuated: %d\n\n", universe.Evacuated)

		if universe.settings.MinimumStepTime >= 0 {
			// 指定時間が経過するまで待つ
			sleep := universe.settings.MinimumStepTime - int(time.Now().Sub(universe.lastStep).Milliseconds())
			if sleep > 0 {
				time.Sleep(time.Duration(sleep) * time.Millisecond)
			}
		} else {
			fmt.Scanln()
		}

		universe.StepCount++
		if universe.StepCount >= universe.cycles[universe.CycleCount%len(universe.cycles)].StepCount {
			universe.CycleCount++
			universe.NeedsCycleStart = true
		}
		universe.syncer.Done()
	}

	// MQTTクライアントの設定
	opts := MQTT.NewClientOptions().AddBroker(universe.settings.BrokerAddress).SetClientID(xid.New().String())
	opts.OnConnect = func(client MQTT.Client) {
		if token := client.Subscribe(fmt.Sprintf("aria/attend/%s", universe.settings.UniverseID), 0, attendRecieved); token.Wait() && token.Error() != nil {
			panic(token.Error())
		}
		if token := client.Subscribe(fmt.Sprintf("aria/prepared/%s", universe.settings.UniverseID), 0, preparedRecieved); token.Wait() && token.Error() != nil {
			panic(token.Error())
		}
		if token := client.Subscribe(fmt.Sprintf("aria/persons/%s", universe.settings.UniverseID), 0, stepRecieved); token.Wait() && token.Error() != nil {
			panic(token.Error())
		}
		fmt.Printf("[Universe] Initialized (%s)\n", opts.ClientID)
		syncer.Done()
	}

	// MQTTブローカーに接続
	universe.client = MQTT.NewClient(opts)
	if token := universe.client.Connect(); token.Wait() && token.Error() != nil {
		panic(token.Error())
	}

	return &syncer
}

func (universe *UniverseModule) Uninitialize() {
	universe.client.Disconnect(250)
	fmt.Println("[Universe] Uninitialize")
}

// サイクルの開始をPublish
func (universe *UniverseModule) PublishCycle() *sync.WaitGroup {
	if !universe.NeedsCycleStart {
		fmt.Printf("TODO : argument error\n")
		return &universe.syncer
	}

	universe.syncer.Add(1)

	universe.StepCount = 0
	for _, personModule := range universe.personModules {
		personModule.IsFinished = false
	}
	bytes, _ := json.Marshal(aria_utility_mqtt.CycleEntity{
		AnnounceStep: universe.cycles[universe.CycleCount%len(universe.cycles)].AnnounceStep,
	})
	if token := universe.client.Publish(fmt.Sprintf("aria/cycle/%s", universe.settings.UniverseID), 0, false, bytes); token.Wait() && token.Error() != nil {
		panic(token.Error())
	}
	// fmt.Printf("--- Cycle %d Start (Annnounce %d Step)---\n", universe.CycleCount, universe.cycles[universe.CycleCount%len(universe.cycles)].AnnounceStep)
	universe.lastStep = time.Now()

	return &universe.syncer
}

// ステップの開始をPublish
func (universe *UniverseModule) PublishStep() *sync.WaitGroup {
	if universe.NeedsCycleStart {
		fmt.Printf("TODO : argument error\n")
		return &universe.syncer
	}

	universe.syncer.Add(1)

	for _, personModule := range universe.personModules {
		personModule.IsFinished = false
	}
	universe.Persons = universe.Persons[:0]
	bytes, _ := json.Marshal(aria_utility_mqtt.CountEntity{
		Count: universe.StepCount,
	})
	if token := universe.client.Publish("/flood/count", 0, false, bytes); token.Wait() && token.Error() != nil {
		panic(token.Error())
	}
	// fmt.Printf("--- Step %d Start (%d ms)---\n", universe.StepCount, time.Now().Sub(universe.lastStep).Milliseconds())
	universe.lastStep = time.Now()

	return &universe.syncer
}
