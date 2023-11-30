package main

import (
	"aria_module_routing"
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
	var routingModule aria_module_routing.RoutingModule
	routingModule.Initialize(settings, settings.Nodes[0]).Wait()
	defer routingModule.Uninitialize()

	// 入力があったら終了する
	fmt.Scanln()
}
