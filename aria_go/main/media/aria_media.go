package main

import (
	"aria_module_media"
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
	var mediaModule aria_module_media.MediaModule
	mediaModule.Initialize(settings, settings.Potentials[0]).Wait()
	defer mediaModule.Uninitialize()

	// 入力があったら終了する
	fmt.Scanln()
}
