package main

import (
	"aria_module_person"
	"aria_utility_settings"
	"flag"
	"fmt"
	"os"
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
	var personModule aria_module_person.PersonModule
	personModule.Initialize(settings, settings.Nodes[0]).Wait()
	defer personModule.Uninitialize()

	// 入力があったら終了する
	fmt.Scanln()
}
