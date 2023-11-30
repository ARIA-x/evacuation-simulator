package aria_module_potential

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"math"
	"math/rand"
	"os"
	"strconv"
	"strings"
	"sync"

	"aria_utility_mqtt"
	"aria_utility_settings"

	"github.com/MasterOfBinary/go-opencl/opencl"
	MQTT "github.com/eclipse/paho.mqtt.golang"
	"github.com/rs/xid"
)

// Personのファイルデータ
type PersonData struct {
	X           int
	Y           int
	PrepareTime int
	Speed       float32
	Alpha       float32 // 移動時に直進する要素
	Acquisition float64 // 警報の取得率（パーソン側）
}

// Personエージェント
type Person struct {
	X           int
	Y           int
	LastX       float32
	LastY       float32
	PowerX      float32
	PowerY      float32
	Status      int
	PrepareTime int
	Data        PersonData
}

// JSON形式のポテンシャルマップのファイルエンティティ
type PotentialJsonEntity struct {
	X         int     `json:"X"`
	Y         int     `json:"Y"`
	Potential float64 `json:"Potential"`
	DR        float64 `json:"dr"`
}

// Potentialモジュール
type PotentialModule struct {
	client             MQTT.Client
	PotentialMap       [][]float64   // ポテンシャルマップ（1/3）：外的要因マップ（１枚）
	DisasterMaps       [][][]float64 // ポテンシャルマップ（2/3）：災害要因マップ（複数）
	InternalMap        []float32     // ポテンシャルマップ（3/3）：内的要因マップ（パーソン毎）
	DisasterLabels     [][]int       // 対象の時間に考慮するマップ
	ObjectMap          [][]int       // 壁(1)と避難所(2)を保持するマップ（衝突判定のために必要）
	ResultMap          [][]float64   // 最終的なポテンシャルマップ（画面出力を考えないのであれば不要）
	context            opencl.Context
	commandQueue       opencl.CommandQueue
	kernel             opencl.Kernel
	personValuesBuffer opencl.Buffer
	personParamsBuffer opencl.Buffer
	potentialsBuffer   opencl.Buffer
	internalsBuffer    opencl.Buffer
	objectsBuffer      opencl.Buffer
}

func (module *PotentialModule) Initialize(settings aria_utility_settings.SettingEntity, potentialEntity aria_utility_settings.SettingPotentialEntity) *sync.WaitGroup {
	syncer := sync.WaitGroup{}
	syncer.Add(1)

	universeID := settings.UniverseID
	settingMesh := potentialEntity.MeshSize

	// 内部設定値
	moduleID := ""
	personIDFrom := 0
	personIDTo := 0
	mapWidth := int(math.Ceil(settings.MapWidth / settingMesh))
	mapHeight := int(math.Ceil(settings.MapHeight / settingMesh))

	// 外的要因ポテンシャルマップの初期化
	module.PotentialMap = make([][]float64, mapWidth)
	module.ObjectMap = make([][]int, mapWidth)
	for x := 0; x < mapWidth; x++ {
		module.PotentialMap[x] = make([]float64, mapHeight)
		module.ObjectMap[x] = make([]int, mapHeight)
		for y := 0; y < mapHeight; y++ {
			module.PotentialMap[x][y] = 0.0
			module.ObjectMap[x][y] = 0
		}
	}
	// 外的要因ポテンシャルマップを読み込み
	for _, externalEntity := range potentialEntity.ExternalMaps {
		if externalEntity.IsJSON {
			// JSON形式の場合
			if buffer, err := ioutil.ReadFile(externalEntity.FilePath); err == nil {
				var externalPotentialJson []PotentialJsonEntity
				json.Unmarshal(buffer, &externalPotentialJson)
				for x := 0; x < mapWidth; x++ {
					for y := 0; y < mapHeight; y++ {
						maxValue := 0.0
						for _, item := range externalPotentialJson {
							dx := float64(x)*settingMesh - float64(item.X)
							dy := float64(y)*settingMesh - float64(item.Y)
							length := math.Sqrt(dx*dx + dy*dy)
							value := item.Potential * math.Exp(-math.Pow(length/item.DR, 2))
							if math.Abs(maxValue) < math.Abs(value) {
								maxValue = value
							}
						}
						module.PotentialMap[x][y] += maxValue
					}
				}
				if externalEntity.IsWall {
					for _, item := range externalPotentialJson {
						module.ObjectMap[int(float64(item.X)/settingMesh)][int(float64(item.Y)/settingMesh)] = 1
					}
				}
				if externalEntity.IsShelter {
					for _, item := range externalPotentialJson {
						module.ObjectMap[int(float64(item.X)/settingMesh)][int(float64(item.Y)/settingMesh)] = 2
					}
				}
			}
		} else {
			// CSV形式の場合
			if buffer, err := os.Open(externalEntity.FilePath); err == nil {
				reader := csv.NewReader(buffer)
				reader.FieldsPerRecord = -1
				y := 0
				for {
					line, e := reader.Read()
					if e == io.EOF {
						break
					}
					for x := 0; x < len(line); x++ {
						value, _ := strconv.ParseFloat(line[x], 64)
						module.PotentialMap[x][y] += value
					}
					y++
				}
			}
		}
	}

	// 災害要因ポテンシャルマップを読み込み
	if len(potentialEntity.DisasterMaps) > 0 {
		module.DisasterMaps = make([][][]float64, len(potentialEntity.DisasterMaps))
		module.DisasterLabels = make([][]int, len(potentialEntity.DisasterMaps))
		for no, disasterEntity := range potentialEntity.DisasterMaps {
			module.DisasterMaps[no] = make([][]float64, mapWidth)
			for x := 0; x < mapWidth; x++ {
				module.DisasterMaps[no][x] = make([]float64, mapHeight)
				for y := 0; y < mapHeight; y++ {
					module.DisasterMaps[no][x][y] = 0.0
				}
			}
			if buffer, err := os.Open(disasterEntity.FilePath); err == nil {
				reader := csv.NewReader(buffer)
				reader.FieldsPerRecord = -1
				y := 0
				for {
					line, e := reader.Read()
					if e == io.EOF {
						break
					}
					for x := 0; x < len(line); x++ {
						value, _ := strconv.ParseFloat(line[x], 64)
						module.DisasterMaps[no][x][y] += value
					}
					y++
				}
			}
			module.DisasterLabels[no] = disasterEntity.Labels
		}
	}

	// 結果マップにポテンシャルを転写しておく
	module.ResultMap = make([][]float64, mapWidth)
	for x := 0; x < mapWidth; x++ {
		module.ResultMap[x] = make([]float64, mapHeight)
		for y := 0; y < mapHeight; y++ {
			module.ResultMap[x][y] = module.PotentialMap[x][y]
		}
	}

	// 初期のパーソンの配列
	personDatas := []PersonData{}

	// パーソンの配列
	var persons map[int]*Person

	// パーソン設定ファイルの読込－－－－－－－－－－－－－－－－－－－－－－－－－－－－－－－－－－－－－－－－

	// ファイルを開く
	file, _ := os.Open(potentialEntity.PersonFilePath)
	reader := csv.NewReader(file)
	reader.FieldsPerRecord = -1

	// 設定ファイルの内容を解析する
	reader.Read()
	for {
		line, e := reader.Read()
		if e == io.EOF {
			break
		}

		x, _ := strconv.Atoi(line[0])
		y, _ := strconv.Atoi(line[1])
		prepareTime, _ := strconv.Atoi(line[3])
		speed, _ := strconv.ParseFloat(line[4], 64)
		acquisition := 1.0
		if len(line) > 14 {
			acquisition, _ = strconv.ParseFloat(line[14], 64)
		}

		// パーソンの生成
		personData := PersonData{
			X:           int(float64(x) / settingMesh),
			Y:           int(float64(y) / settingMesh),
			PrepareTime: prepareTime,
			Speed:       float32(speed / settingMesh),
			Alpha:       1.0 / 128.0,
			Acquisition: acquisition,
		}
		personDatas = append(personDatas, personData)
	}

	// 内的要因マップ
	fmt.Printf("[Potential] Loading Internal Maps ")
	module.InternalMap = make([]float32, len(personDatas)*mapWidth*mapHeight)
	for i := 0; i < len(persons)*mapWidth*mapHeight; i++ {
		module.InternalMap[i] = 0
	}
	for index := 0; index < len(personDatas); index++ {
		if potentialEntity.InternalMaps != "" {
			if buffer, err := os.Open(fmt.Sprintf(potentialEntity.InternalMaps, index)); err == nil {
				reader := csv.NewReader(buffer)
				reader.FieldsPerRecord = -1
				y := 0
				for {
					line, e := reader.Read()
					if e == io.EOF {
						break
					}
					for x := 0; x < len(line); x++ {
						value, _ := strconv.ParseFloat(line[x], 64)
						module.InternalMap[index*mapWidth*mapHeight+y*mapWidth+x] = float32(value)
					}
					y++
				}
			}
		}

		if index%100 == 0 {
			fmt.Printf(".")
		}
	}
	fmt.Printf("\n")
	// －－－－－－－－－－－－－－－－－－－－－－－－－－－－－－－－－－－－－－－－

	// GPUの準備－－－－－－－－－－－－－－－－－－－－－－－－－－－－－－－－－－－－－－－－
	const programCode = `
kernel void calc(global int* personValues, global float* personParams, global float* potentials, global float* internals, global int* objects)
{
	size_t index = get_global_id(0);

	int x, y;
	float power, f1, f2, f3, f4, f5, f6, f7, f8, dx, dy, lastLength, l, wx, wy, ax, ay;

	if (personValues[index*4+3] == 2) {
		if (personValues[index*4+2] > 0) {
			personValues[index*4+2]--;
			return;
		} else {
			personValues[index*4+3] = 3;
		}
	}

	if (personValues[index*4+3] != 3) {
		return;
	}
	
	power = personParams[index*6+0];
	while (1) {
		x = personValues[index*4+0];
		y = personValues[index*4+1];
		f1 = 0.0;
		f2 = 0.0;
		f3 = 0.0;
		f4 = 0.0;
		f5 = 0.0;
		f6 = 0.0;
		f7 = 0.0;
		f8 = 0.0;

		if (x+1 < width) {
			f1 = potentials[y*width+x] - potentials[(y+0)*width+(x+1)] - internals[index*width*height+(y+0)*width+(x+1)];
			if (y-1 >= 0) {
				f2 = (potentials[y*width+x] - potentials[(y-1)*width+(x+1)] - internals[index*width*height+(y-1)*width+(x+1)]) / 2;
			}
			if (y+1 < height) {
				f8 = (potentials[y*width+x] - potentials[(y+1)*width+(x+1)] - internals[index*width*height+(y+1)*width+(x+1)]) / 2;
			}
		}
		if (y-1 >= 0) {
			f3 = potentials[y*width+x] - potentials[(y-1)*width+(x+0)] - internals[index*width*height+(y-1)*width+(x+0)];
		}
		if (y+1 < height) {
			f7 = potentials[y*width+x] - potentials[(y+1)*width+(x+0)] - internals[index*width*height+(y+1)*width+(x+0)];
		}
		if (x-1 >= 0) {
			f5 = potentials[y*width+x] - potentials[(y+0)*width+(x-1)] - internals[index*width*height+(y+0)*width+(x-1)];
			if (y-1 >= 0) {
				f4 = (potentials[y*width+x] - potentials[(y-1)*width+(x-1)] - internals[index*width*height+(y-1)*width+(x-1)]) / 2;
			}
			if (y+1 < height) {
				f6 = (potentials[y*width+x] - potentials[(y+1)*width+(x-1)] - internals[index*width*height+(y+1)*width+(x-1)]) / 2;
			}
		}
		dx = f2 + f1 + f8 - f4 - f5 - f6;
		dy = f6 + f7 + f8 - f4 - f3 - f2;
		lastLength = sqrt(personParams[index*6+2]*personParams[index*6+2] + personParams[index*6+3]*personParams[index*6+3]);
		if (lastLength > 0) {
			dx += personParams[index*6+2] / lastLength * personParams[index*6+1];
			dy += personParams[index*6+3] / lastLength * personParams[index*6+1];
		}
		personParams[index*6+2] = dx;
		personParams[index*6+3] = dy;

		l = sqrt(dx*dx + dy*dy);
		dx *= power / l;
		dy *= power / l;

		wx = 1.0 - personParams[index*6+4];
		wy = 1.0 - personParams[index*6+5];
		ax = dx < 0 ? -dx : dx;
		ay = dy < 0 ? -dy : dy;
		if (ax/wx >= ay/wy && ax/wx >= 1.0) {
			if (0 <= x+(int)(dx/ax) && x+(int)(dx/ax) < width && objects[y*width+(x+(int)(dx/ax))] != 1) {
				personValues[index*4+0] = x + (int)(dx/ax);
			}
			personParams[index*6+4] = 0;
			personParams[index*6+5] += ay / ax * wx;
			power -= sqrt(wx*wx + ay/ax*wx*ay/ax*wx);
		} else if (ax/wx < ay/wy && ay/wy >= 1.0) {
			if (0 <= y+(int)(dy/ay) && y+(int)(dy/ay) < height && objects[(y+(int)(dy/ay))*width+x] != 1) {
				personValues[index*4+1] = y + (int)(dy/ay);
			}
			personParams[index*6+4] += ax / ay * wy;
			personParams[index*6+5] = 0;
			power -= sqrt(ax/ay*wy*ax/ay*wy + wy*wy);
		} else {
			personParams[index*6+4] += ax;
			personParams[index*6+5] += ay;
			power -= sqrt(ax*ax + ay*ay);
			break;
		}

		if (objects[personValues[index*4+1]*width+personValues[index*4+0]] == 2) {
			personValues[index*4+3] = 7;
			break;
		}
	}
}
`
	module.context, module.commandQueue, module.kernel = PrepareGPU(strings.Replace(strings.Replace(programCode, "width", strconv.Itoa(mapWidth), -1), "height", strconv.Itoa(mapHeight), -1))
	module.personValuesBuffer = CreateBuffer(module.context, module.kernel, 0, 4*uint64(len(personDatas))*4)
	module.personParamsBuffer = CreateBuffer(module.context, module.kernel, 1, 4*uint64(len(personDatas))*6)
	module.potentialsBuffer = CreateBuffer(module.context, module.kernel, 2, 4*uint64(mapWidth*mapHeight))
	module.internalsBuffer = CreateBuffer(module.context, module.kernel, 3, 4*uint64(len(personDatas)*mapWidth*mapHeight))
	module.objectsBuffer = CreateBuffer(module.context, module.kernel, 4, 4*uint64(mapWidth*mapHeight))
	if err := module.commandQueue.EnqueueWriteBuffer(module.internalsBuffer, true, module.InternalMap); err != nil {
		panic(err)
	}

	// パーソンエージェントの参加完了
	var registeredRecieved MQTT.MessageHandler = func(client MQTT.Client, msg MQTT.Message) {
		var entity aria_utility_mqtt.RegisteredEntity
		json.Unmarshal(msg.Payload(), &entity)

		personIDFrom = entity.From
		personIDTo = entity.To
		fmt.Printf("[Potential] Registered : %s (%d to %d)\n", entity.ID, personIDFrom, personIDTo)
		syncer.Done()
	}

	// サイクルの開始
	var cycleRecieved MQTT.MessageHandler = func(client MQTT.Client, msg MQTT.Message) {
		var entity aria_utility_mqtt.CycleEntity
		json.Unmarshal(msg.Payload(), &entity)

		// パーソンの新規作成
		persons = make(map[int]*Person)
		index := 0
		for i := personIDFrom; i < personIDTo; i++ {
			persons[i] = &Person{
				X:           personDatas[index].X,
				Y:           personDatas[index].Y,
				Data:        personDatas[index],
				PowerX:      0.0,
				PowerY:      0.0,
				LastX:       0.0,
				LastY:       0.0,
				Status:      0,
				PrepareTime: personDatas[index].PrepareTime,
			}
			index++
		}

		// 準備完了をPublish
		results := []aria_utility_mqtt.AllEntity{}
		for id, person := range persons {
			results = append(results, aria_utility_mqtt.AllEntity{
				Count:      0,
				ID:         id,
				X:          float64(person.X) * settingMesh,
				Y:          float64(person.Y) * settingMesh,
				Status:     person.Status,
				InfoAccess: 0,
			})
		}
		bytes, _ := json.Marshal(aria_utility_mqtt.PreparedEntity{
			ID:      moduleID,
			Persons: results,
		})
		if token := client.Publish(fmt.Sprintf("aria/prepared/%s", universeID), 0, false, bytes); token.Wait() && token.Error() != nil {
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

		// 現在の災害マップ、描画用マップ
		for x := 0; x < mapWidth; x++ {
			for y := 0; y < mapHeight; y++ {
				module.ResultMap[x][y] = 0
			}
		}
		for no, label := range module.DisasterLabels {
			for _, value := range label {
				if value == entity.Count {
					for x := 0; x < mapWidth; x++ {
						for y := 0; y < mapHeight; y++ {
							module.ResultMap[x][y] += module.DisasterMaps[no][x][y]
						}
					}
					break
				}
			}
		}
		personValues := make([]int32, len(persons)*4)
		personParams := make([]float32, len(persons)*6)
		for index, person := range persons {
			depthDecelerate := math.Max(0, 0.7-math.Max(0, module.ResultMap[person.X][person.Y])) / 0.7
			speedDecelerate := math.Max(0, 2.5-math.Max(0, module.ResultMap[person.X][person.Y])*3.5714285714285714285714285714286) / 2.5
			personValues[index*4+0] = int32(person.X)
			personValues[index*4+1] = int32(person.Y)
			personValues[index*4+2] = int32(person.PrepareTime)
			personValues[index*4+3] = int32(person.Status)
			personParams[index*6+0] = person.Data.Speed * float32(depthDecelerate) * float32(speedDecelerate)
			personParams[index*6+1] = person.Data.Alpha
			personParams[index*6+2] = person.LastX
			personParams[index*6+3] = person.LastY
			personParams[index*6+4] = person.PowerX
			personParams[index*6+5] = person.PowerY
		}
		potentials := make([]float32, mapWidth*mapHeight)
		objects := make([]int32, mapWidth*mapHeight)
		personCounts := make([]int, mapWidth*mapHeight)
		for x := 0; x < mapWidth; x++ {
			for y := 0; y < mapHeight; y++ {
				module.ResultMap[x][y] += module.PotentialMap[x][y]
				potentials[y*mapWidth+x] = float32(module.ResultMap[x][y])
				objects[y*mapWidth+x] = int32(module.ObjectMap[x][y])
				personCounts[y*mapWidth+x] = 0
			}
		}
		for _, person := range persons {
			if person.Status != 7 {
				potentials[person.Y*mapWidth+person.X] += 0.0075
				personCounts[person.Y*mapWidth+person.X]++
			}
		}
		// TODO : ステップ開始時点での人数で入場禁止を決めるので、一時的に４人以上が入る可能性がある
		for i := 0; i < mapWidth*mapHeight; i++ {
			if objects[i] == 0 && personCounts[i] >= 4 {
				objects[i] = 1
			}
		}

		if err := module.commandQueue.EnqueueWriteBuffer(module.personValuesBuffer, true, personValues); err != nil {
			panic(err)
		}
		if err := module.commandQueue.EnqueueWriteBuffer(module.personParamsBuffer, true, personParams); err != nil {
			panic(err)
		}
		if err := module.commandQueue.EnqueueWriteBuffer(module.potentialsBuffer, true, potentials); err != nil {
			panic(err)
		}
		if err := module.commandQueue.EnqueueWriteBuffer(module.objectsBuffer, true, objects); err != nil {
			panic(err)
		}
		if err := module.commandQueue.EnqueueNDRangeKernel(module.kernel, 1, []uint64{uint64(len(persons))}); err != nil {
			panic(err)
		}
		module.commandQueue.Flush()
		module.commandQueue.Finish()
		if err := module.commandQueue.EnqueueReadBuffer(module.personValuesBuffer, true, personValues); err != nil {
			panic(err)
		}
		if err := module.commandQueue.EnqueueReadBuffer(module.personParamsBuffer, true, personParams); err != nil {
			panic(err)
		}

		for index, person := range persons {
			person.X = int(personValues[index*4+0])
			person.Y = int(personValues[index*4+1])
			person.PrepareTime = int(personValues[index*4+2])
			person.Status = int(personValues[index*4+3])
			person.LastX = personParams[index*6+2]
			person.LastY = personParams[index*6+3]
			person.PowerX = personParams[index*6+4]
			person.PowerY = personParams[index*6+5]
		}

		// 結果をPublish
		results := []aria_utility_mqtt.AllEntity{}
		for id, person := range persons {
			results = append(results, aria_utility_mqtt.AllEntity{
				Count:      entity.Count,
				ID:         id,
				X:          float64(person.X) * settingMesh,
				Y:          float64(person.Y) * settingMesh,
				Status:     person.Status,
				InfoAccess: 0,
			})
		}
		bytes, _ := json.Marshal(aria_utility_mqtt.StepEntity{
			ID:      moduleID,
			Persons: results,
		})
		if token := client.Publish(fmt.Sprintf("aria/persons/%s", universeID), 0, false, bytes); token.Wait() && token.Error() != nil {
			panic(token.Error())
		}
	}

	// メディア（地震の揺れ、広報車など）
	var mediaAleatRecieved MQTT.MessageHandler = func(client MQTT.Client, msg MQTT.Message) {
		if !module.client.IsConnected() {
			return
		}

		var entity aria_utility_mqtt.MediaEntity
		json.Unmarshal(msg.Payload(), &entity)

		// 各パーソンの処理を実行
		for _, person := range persons {
			if person.Status == 0 && math.Sqrt((float64(person.X)-entity.X/settingMesh)*(float64(person.X)-entity.X/settingMesh)+(float64(person.Y)-entity.Y/settingMesh)*(float64(person.Y)-entity.Y/settingMesh)) < entity.Size/settingMesh && rand.Float64() < entity.Acquisition*person.Data.Acquisition {
				person.Status = 2
			}
		}
	}

	// MQTTクライアントの設定
	opts := MQTT.NewClientOptions().AddBroker(settings.BrokerAddress).SetClientID(xid.New().String())
	opts.OnConnect = func(client MQTT.Client) {

		// モジュールIDをランダムに設定
		moduleID = xid.New().String()

		// MQTTのサブスクライブ
		if token := client.Subscribe(fmt.Sprintf("aria/registered/%s/%s", universeID, moduleID), 0, registeredRecieved); token.Wait() && token.Error() != nil {
			panic(token.Error())
		}
		if token := client.Subscribe(fmt.Sprintf("aria/cycle/%s", universeID), 0, cycleRecieved); token.Wait() && token.Error() != nil {
			panic(token.Error())
		}
		if token := client.Subscribe("/flood/count", 0, countRecieved); token.Wait() && token.Error() != nil {
			panic(token.Error())
		}
		if token := client.Subscribe(fmt.Sprintf("aria/media/%s", universeID), 0, mediaAleatRecieved); token.Wait() && token.Error() != nil {
			panic(token.Error())
		}

		// Universeモジュールに参加をPublish
		bytes, _ := json.Marshal(aria_utility_mqtt.AttendEntity{
			ID:    moduleID,
			Count: len(personDatas),
		})
		if token := client.Publish(fmt.Sprintf("aria/attend/%s", universeID), 0, false, bytes); token.Wait() && token.Error() != nil {
			panic(token.Error())
		}

		fmt.Printf("[Potential] Initialized (%s)\n", opts.ClientID)
	}

	// MQTTブローカーに接続
	module.client = MQTT.NewClient(opts)
	if token := module.client.Connect(); token.Wait() && token.Error() != nil {
		panic(token.Error())
	}

	return &syncer
}

func (module *PotentialModule) Uninitialize() {
	module.client.Disconnect(250)
	module.internalsBuffer.Release()
	module.objectsBuffer.Release()
	module.personParamsBuffer.Release()
	module.personValuesBuffer.Release()
	module.potentialsBuffer.Release()
	module.kernel.Release()
	module.commandQueue.Release()
	module.context.Release()
	fmt.Println("[Potential] Uninitialize")
}

// PrepareGPU GPUを準備する
func PrepareGPU(programCode string) (opencl.Context, opencl.CommandQueue, opencl.Kernel) {

	// すべてのプラットフォームを取得
	platforms, err := opencl.GetPlatforms()
	if err != nil {
		panic(err)
	}

	// すべてのプラットフォームを確認し、利用できるデバイスを取得する
	foundDevice := false
	var device opencl.Device
	var name string
	for _, curPlatform := range platforms {

		// プラットフォームの情報を取得する
		err = curPlatform.GetInfo(opencl.PlatformName, &name)
		if err != nil {
			panic(err)
		}

		// デバイスの情報を取得する
		var devices []opencl.Device
		devices, err = curPlatform.GetDevices(opencl.DeviceTypeAll)
		if err != nil {
			panic(err)
		}

		// 最初のデバイスを利用するために保持する
		if len(devices) > 0 && !foundDevice {
			var available bool
			err = devices[0].GetInfo(opencl.DeviceAvailable, &available)
			if err == nil && available {
				device = devices[0]
				foundDevice = true
			}
		}

		// プラットフォームとデバイスの名前を出力する（すべて）
		fmt.Printf("Name: %v, devices: %v, version: %v\n", name, len(devices), curPlatform.GetVersion())
	}
	if !foundDevice {
		panic("No device found")
	}

	// コンテキストを生成する
	var context opencl.Context
	context, err = device.CreateContext()
	if err != nil {
		panic(err)
	}

	// キューを生成する
	var commandQueue opencl.CommandQueue
	commandQueue, err = context.CreateCommandQueue(device)
	if err != nil {
		panic(err)
	}

	// プログラムを生成する
	var program opencl.Program
	program, err = context.CreateProgramWithSource(programCode)
	if err != nil {
		panic(err)
	}

	// プログラムをコンパイルする
	var log string
	err = program.Build(device, &log)
	if err != nil {
		fmt.Println(log)
		panic(err)
	}

	// カーネルを生成する
	kernel, err := program.CreateKernel("calc") // カーネル内のメソッド名
	if err != nil {
		panic(err)
	}

	return context, commandQueue, kernel
}

// CreateBuffer GPUのメモリを用意する
func CreateBuffer(context opencl.Context, kernel opencl.Kernel, index uint32, size uint64) opencl.Buffer {
	buffer, err := context.CreateBuffer([]opencl.MemFlags{opencl.MemReadWrite}, size)
	if err != nil {
		panic(err)
	}
	if err = kernel.SetArg(index, buffer.Size(), &buffer); err != nil {
		panic(err)
	}
	return buffer
}

func sqrt(value float32) float32 {
	return float32(math.Sqrt(float64(value)))
}

func abs(value float32) float32 {
	return float32(math.Abs(float64(value)))
}
