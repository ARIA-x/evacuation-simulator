package main

import (
	"aria_module_potential"
	"aria_utility_settings"
	"flag"
	"fmt"
	"os"
)

func main() {
	settingFileName := "../../../data/settings_potential.json"

	// コマンドライン引数から設定ファイル名を取得する
	flag.Parse()
	if len(flag.Args()) > 0 {
		settingFileName = flag.Args()[0]
	}

	// 設定ファイル読み込み
	settings := aria_utility_settings.LoadSettings(settingFileName)
	os.Chdir(settings.RootPath)

	// モジュール起動
	var potentialModule aria_module_potential.PotentialModule
	potentialModule.Initialize(settings, settings.Potentials[0]).Wait()
	defer potentialModule.Uninitialize()

	// 入力があったら終了する
	fmt.Scanln()
}
