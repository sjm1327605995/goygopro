// Copyright 2014 Hajime Hoshi
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package main

import (
	"flag"
	"log"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/sjm1327605995/goygopro/game"
)

var cpuProfile = flag.String("cpuprofile", "", "write cpu profile to file")

func main() {
	flag.Parse()
	//if *cpuProfile != "" {
	//	f, err := os.Create(*cpuProfile)
	//	if err != nil {
	//		log.Fatal(err)
	//	}
	//	w := bufio.NewWriter(f)
	//	if err := pprof.StartCPUProfile(w); err != nil {
	//		log.Fatal(err)
	//	}
	//	defer func() {
	//		if err := w.Flush(); err != nil {
	//			log.Fatal(err)
	//		}
	//	}()
	//	defer pprof.StopCPUProfile()
	//}

	ebiten.SetWindowTitle("Ebitengine")
	ebiten.SetWindowResizingMode(ebiten.WindowResizingModeEnabled)
	if err := ebiten.RunGame(game.NewGame()); err != nil {
		log.Fatal(err)
	}
}
