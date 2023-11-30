package aria_utility_floods

import (
	"aria_utility_settings"
	"encoding/csv"
	"fmt"
	"io"
	"os"
	"strconv"
)

// LoadFloods 洪水情報を読み込む
func LoadFloods(settings aria_utility_settings.SettingEntity, floodWidth int, floodHeight int, stepCount int) ([][]float64, float64, float64) {
	total := 0.0
	max := 0.0
	floods := make([][]float64, floodWidth)
	for i := 0; i < len(floods); i++ {
		floods[i] = make([]float64, floodHeight)
	}

	// 洪水ファイルの読み込み－－－－－－－－－－－－－－－－－－－－－－－－－－－－－－－－－－－－－－－－
	file, _ := os.Open(fmt.Sprintf(settings.FloodFilePath, stepCount))
	reader := csv.NewReader(file)
	reader.FieldsPerRecord = -1
	reader.Read()
	for {
		line, e := reader.Read()
		if e == io.EOF || len(line) < 3 {
			break
		}

		x, _ := strconv.ParseFloat(line[1], 64)
		y, _ := strconv.ParseFloat(line[2], 64)
		depth, _ := strconv.ParseFloat(line[3], 64)

		total += depth
		if max < depth {
			max = depth
		}

		px := int(x / settings.FloodMeshSize)
		py := int(y / settings.FloodMeshSize)
		if px >= 0 && px < floodWidth && py >= 0 && py < floodHeight {
			floods[px][py] = depth
		}
	}
	// －－－－－－－－－－－－－－－－－－－－－－－－－－－－－－－－－－－－－－－－

	return floods, total, max
}
