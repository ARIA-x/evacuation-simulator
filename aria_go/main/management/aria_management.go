package main

import (
	"aria_module_media"
	"aria_module_person"
	"aria_module_potential"
	"aria_module_routing"
	"aria_module_universe"
	"aria_utility_floods"
	"aria_utility_nodes"
	"aria_utility_settings"
	"flag"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"math"
	"os"
	"time"
)

func main() {
	// settingFileName := "../../../data/settings_dummy.json"
	settingFileName := "../../../data/settings_potential.json"
	// settingFileName := "../../../data/settings.json"

	// コマンドライン引数から設定ファイル名を取得する
	flag.Parse()
	if len(flag.Args()) > 0 {
		settingFileName = flag.Args()[0]
	}

	settings := aria_utility_settings.LoadSettings(settingFileName)
	potentialMeshSize := 0.0

	os.Chdir(settings.RootPath)

	var universeModule aria_module_universe.UniverseModule
	universeModule.Initialize(settings).Wait()
	defer universeModule.Uninitialize()

	var routingModule aria_module_routing.RoutingModule
	if len(settings.Nodes) > 0 {
		routingModule.Initialize(settings, settings.Nodes[0]).Wait()
		defer routingModule.Uninitialize()
	}

	var personModule aria_module_person.PersonModule
	if len(settings.Nodes) > 0 {
		personModule.Initialize(settings, settings.Nodes[0]).Wait()
		defer personModule.Uninitialize()
	}

	var potentialModule aria_module_potential.PotentialModule
	if len(settings.Potentials) > 0 {
		potentialMeshSize = settings.Potentials[0].MeshSize
		potentialModule.Initialize(settings, settings.Potentials[0]).Wait()
		defer potentialModule.Uninitialize()
	}

	var mediaModule aria_module_media.MediaModule
	if len(settings.Potentials) > 0 {
		mediaModule.Initialize(settings, settings.Potentials[0]).Wait()
		defer mediaModule.Uninitialize()
	}

	// 座標出力用CSVファイル
	if _, err := os.Stat("./result"); os.IsNotExist(err) {
		os.Mkdir("./result", 0777)
	}
	if _, err := os.Stat("./result/images"); os.IsNotExist(err) {
		os.Mkdir("./result/images", 0777)
	}
	file, _ := os.Create(fmt.Sprintf("./result/persons.csv"))
	defer file.Close()
	fmt.Fprintf(file, "Cycle,Step,Index,ID,X,Y,Status,Access\n")

	// マップ画像の生成
	imageZoom := 547.55 / math.Max(settings.MapWidth, settings.MapHeight)
	imageWidth := int(math.Ceil(settings.MapWidth * imageZoom))
	imageHeight := int(math.Ceil(settings.MapHeight * imageZoom))
	rect := image.Rect(0, 0, imageWidth, imageHeight)
	mapImage := image.NewRGBA(rect)
	for x := 0; x < imageWidth; x++ {
		for y := 0; y < imageHeight; y++ {
			mapImage.Set(x, y, color.RGBA{255, 255, 255, 255})
		}
	}

	// ノードを使ったマップの描画
	if len(settings.Nodes) > 0 {
		nodes := aria_utility_nodes.LoadMap(settings, settings.Nodes[0])
		for _, node := range nodes {
			for _, neighbor := range node.Neighbors {
				for v := 0.0; v < 1; v += 0.01 {
					mapImage.Set(int((node.X*v+neighbor.Node.X*(1-v))*imageZoom+imageZoom*0.5), int((node.Y*v+neighbor.Node.Y*(1-v))*imageZoom+imageZoom*0.5), color.RGBA{224, 224, 224, 255})
				}
			}
		}
		for _, node := range nodes {
			mapImage.Set(int(node.X*imageZoom+imageZoom*0.5), int(node.Y*imageZoom+imageZoom*0.5), color.RGBA{192, 192, 192, 255})
		}
		for _, node := range nodes {
			if node.IsShelter {
				mapImage.Set(int(node.X*imageZoom+imageZoom*0.5), int(node.Y*imageZoom+imageZoom*0.5), color.RGBA{255, 128, 0, 255})
			}
		}
	}

	// サイクルを１回だけ実行
	universeModule.PublishCycle().Wait()
	for !universeModule.NeedsCycleStart {

		// 結果画像の生成
		imageStart := time.Now()
		resultImage := image.NewRGBA(rect)
		draw.Draw(resultImage, rect, mapImage, image.Pt(0, 0), draw.Src)

		// ノードを使った洪水の描画
		if len(settings.Nodes) > 0 {
			floodWidth := int(math.Ceil(settings.MapWidth / settings.FloodMeshSize))
			floodHeight := int(math.Ceil(settings.MapHeight / settings.FloodMeshSize))
			floods, _, _ := aria_utility_floods.LoadFloods(settings, floodWidth, floodHeight, universeModule.StepCount)
			for x := 0; x < imageWidth; x++ {
				for y := 0; y < imageHeight; y++ {
					depth := floods[int(float64(x)/imageZoom/settings.FloodMeshSize)][int(float64(y)/imageZoom/settings.FloodMeshSize)]
					if depth > 0.0 {
						resultImage.Set(x, y, color.RGBA{0, uint8(255 * (1 - depth/2.0)), 255, 255})
					}
				}
			}
		}

		// ポテンシャルを使ったマップの描画
		if len(settings.Potentials) > 0 {
			maxPotential := 1.0
			minPotential := -1.2
			for x := 0; x < imageWidth; x++ {
				for y := 0; y < imageHeight; y++ {
					potential := potentialModule.ResultMap[int(float64(x)/imageZoom/potentialMeshSize)][int(float64(y)/imageZoom/potentialMeshSize)]
					if potential > 0 {
						value := uint8(math.Max(0.0, math.Min(1.0, (maxPotential-potential)/maxPotential))*100 + 150)
						resultImage.Set(x, y, color.RGBA{255, value, value, 255})
					} else {
						value := uint8(math.Max(0.0, math.Min(1.0, (minPotential-potential)/minPotential))*100 + 150)
						resultImage.Set(x, y, color.RGBA{value, value, 255, 255})
					}
					value := potentialModule.ObjectMap[int(float64(x)/imageZoom/potentialMeshSize)][int(float64(y)/imageZoom/potentialMeshSize)]
					if value == 1 {
						resultImage.Set(x, y, color.RGBA{0, 0, 0, 255})
					}
					if value == 2 {
						resultImage.Set(x, y, color.RGBA{255, 128, 0, 255})
					}
				}
			}
			for _, mediaEntity := range settings.Potentials[0].Media {
				currentStep := universeModule.StepCount - mediaEntity.Step
				if 0 <= currentStep && currentStep <= mediaEntity.Duration {
					cx := mediaEntity.Positions[currentStep%len(mediaEntity.Positions)].X / settings.Potentials[0].MeshSize
					cy := mediaEntity.Positions[currentStep%len(mediaEntity.Positions)].Y / settings.Potentials[0].MeshSize
					size := mediaEntity.Size / settings.Potentials[0].MeshSize
					for y := 0; y < imageHeight; y++ {
						for x := 0; x < imageWidth; x++ {
							if (float64(x)-cx)*(float64(x)-cx)+(float64(y)-cy)*(float64(y)-cy) < size*size {
								resultImage.Set(x, y, color.RGBA{192, 255, 192, 255})
							}
						}
					}
				}
			}
		}

		// 全てのパーソンの処理
		for index, person := range universeModule.Persons {

			// データのCSV出力
			fmt.Fprintf(file, "%d,%d,%d,%d,%f,%f,%d,%d\n", universeModule.CycleCount, universeModule.StepCount, index, person.ID, person.X, person.Y, person.Status, person.InfoAccess)

			c := color.RGBA{64, 64, 64, 255}
			switch person.Status {
			case 2:
				c = color.RGBA{128, 128, 0, 255}
				break
			case 3:
				c = color.RGBA{0, 128, 0, 255}
				break
			case 4:
				c = color.RGBA{192, 96, 0, 255}
				break
			case 5:
				c = color.RGBA{192, 0, 192, 255}
				break
			case 6:
				c = color.RGBA{192, 0, 0, 255}
				break
			case 7:
				c = color.RGBA{0, 64, 128, 255}
				break
			}
			x := int((person.X*settings.FloodMeshSize + 0.5) * imageZoom)
			y := int((person.Y*settings.FloodMeshSize + 0.5) * imageZoom)
			resultImage.Set(x, y, c)
			resultImage.Set(x, y-1, c)
			resultImage.Set(x, y+1, c)
			resultImage.Set(x-1, y, c)
			resultImage.Set(x+1, y, c)
			resultImage.Set(x, y-2, c)
			resultImage.Set(x, y+2, c)
			resultImage.Set(x-2, y, c)
			resultImage.Set(x+2, y, c)
		}

		// マップ画像の保存
		imageFile, _ := os.Create(fmt.Sprintf("./result/images/map_%03d_%03d.png", universeModule.CycleCount, universeModule.StepCount))
		png.Encode(imageFile, resultImage)
		imageFile.Close()
		imageFinish := time.Now()

		// ステップの実行
		stepStart := time.Now()
		universeModule.PublishStep().Wait()
		stepFinish := time.Now()

		fmt.Printf("Step %3d Finished | %4d ms | %4d ms | Affected %4d | Evacuated %4d\n", universeModule.StepCount, stepFinish.Sub(stepStart).Milliseconds(), imageFinish.Sub(imageStart).Milliseconds(), universeModule.Affected, universeModule.Evacuated)
	}
}
