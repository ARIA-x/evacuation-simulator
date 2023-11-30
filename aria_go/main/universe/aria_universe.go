package main

import (
	"aria_module_universe"
	"aria_utility_settings"
	"flag"
	"fmt"
	"os"
	"time"
)

func main() {
	settingFileName := "../../../data/settings.json"

	// コマンドライン引数から設定ファイル名を取得する
	flag.Parse()
	if len(flag.Args()) > 0 {
		settingFileName = flag.Args()[0]
	}

	// 設定ファイル読み込み
	settings := aria_utility_settings.LoadSettings(settingFileName)
	os.Chdir(settings.RootPath)

	// モジュール起動
	var universeModule aria_module_universe.UniverseModule
	universeModule.Initialize(settings).Wait()
	defer universeModule.Uninitialize()

	// 入力待ち
	fmt.Scanln()

	// サイクルを連続実行
	for true {
		// 最初のサイクルをPublish
		universeModule.PublishCycle().Wait()
		for !universeModule.NeedsCycleStart {
			stepStart := time.Now()
			universeModule.PublishStep().Wait()
			stepFinish := time.Now()
			fmt.Printf("Step %3d Finished | %4d ms | Affected %4d | Evacuated %4d\n", universeModule.StepCount, stepFinish.Sub(stepStart).Milliseconds(), universeModule.Affected, universeModule.Evacuated)
		}
	}
}
