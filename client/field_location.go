package client

import (
	"math"
)

// Screen coordinate mapping from C++ Irrlicht world coordinates.
// World bounds: X [-1.0, 9.0], Y [-4.0, 4.0]
// Screen: 1024 x 640 (Y down, same as Irrlicht 2D mode)
const (
	worldMinX = -1.0
	worldMaxX = 9.0
	worldMinY = -4.0
	worldMaxY = 4.0
	worldW    = worldMaxX - worldMinX // 10.0
	worldH    = worldMaxY - worldMinY // 8.0
	screenW   = 1024.0
	screenH   = 640.0
	scaleX    = screenW / worldW // 102.4
	scaleY    = screenH / worldH // 80.0
)

func worldToScreen(wx, wy float32) (float32, float32) {
	sx := (wx - float32(worldMinX)) * float32(scaleX)
	sy := (wy - float32(worldMinY)) * float32(scaleY)
	return sx, sy
}

// 这里的「屏幕坐标」是**未投影的桌面平面**，不是最终画面：GUI 层把整块桌面当一个 3D 场景
// 倾斜过去（tenon 的 Scene3D），投影由它负责。所以这套坐标只描述「卡摆在桌上的哪」，
// 与相机角度无关 —— 换角度不必动这里。
//
// C++ 原版用 Irrlicht 相机看同一张桌子：视点 (4.2, 8.0, 7.8)、看向 (4.2, 0, 0)、up 为 Z 轴
// （见 gframe/game.cpp 的 MainLoop）。俯角约 atan(8.0/7.8) ≈ 45.7°，即 GUI 层那边的
// RotateX。卡在世界里是 0.7 x 1.0（gframe/materials.cpp），换算过来就是 CardSize()。

// FieldSize 返回桌面平面的像素尺寸。
func FieldSize() (w, h float32) { return screenW, screenH }

// CardSize 返回一张卡在桌面平面上的像素尺寸（世界里的 0.7 x 1.0）。
func CardSize() (w, h float32) { return 0.7 * scaleX, 1.0 * scaleY }

// ZoneCenter 返回某个区域在桌面平面上的中心像素坐标。
func ZoneCenter(controler int, location uint8, sequence int, rule int) (x, y float32) {
	return zoneCenter(controler, location, sequence, rule)
}

// FieldRule 返回当前决斗规则对应的场地布局（0=旧规则, 1=新大师规则）。
func FieldRule() int {
	if MainGame.DInfo.DuelRule >= 4 {
		return 1
	}
	return 0
}

// zoneCenter returns the screen center of a field zone.
// This is a simplified translation of C++ Materials vertex centers.
func zoneCenter(controler int, location uint8, sequence int, rule int) (x, y float32) {
	var wx, wy float32
	switch location {
	case 0x01: // DECK
		if controler == 0 {
			wx, wy = 7.3, 3.3
		} else {
			wx, wy = 0.6, -3.3
		}
	case 0x02: // HAND
		count := len(MainGame.DField.Hand[controler])
		if controler == 0 {
			if count <= 6 {
				wx = (5.5-0.8*float32(count))/2 + 1.55 + float32(sequence)*0.8
			} else {
				wx = 1.9 + float32(sequence)*4.0/float32(count-1)
			}
			wy = 4.0
		} else {
			if count <= 6 {
				wx = 6.25 - (5.5-0.8*float32(count))/2 - float32(sequence)*0.8
			} else {
				wx = 5.9 - float32(sequence)*4.0/float32(count-1)
			}
			wy = -3.4
		}
	case 0x04: // MZONE
		if controler == 0 {
			if sequence < 5 {
				wx = 1.75 + float32(sequence)*1.1
				wy = 1.4
			} else if sequence == 5 {
				wx, wy = 2.85, 0.0
			} else {
				wx, wy = 5.05, 0.0
			}
		} else {
			if sequence < 5 {
				wx = 6.15 - float32(sequence)*1.1
				wy = -1.4
			} else if sequence == 5 {
				wx, wy = 5.05, 0.0
			} else {
				wx, wy = 2.85, 0.0
			}
		}
	case 0x08: // SZONE
		if controler == 0 {
			if sequence < 5 {
				wx = 1.75 + float32(sequence)*1.1
				wy = 2.6
			} else if sequence == 5 {
				if rule == 0 {
					wx, wy = 0.6, 0.7
				} else {
					wx, wy = 0.6, 2.0
				}
			} else if sequence == 6 {
				if rule == 0 {
					wx, wy = 0.6, 2.0
				} else {
					wx, wy = 0.6, 0.7
				}
			} else {
				if rule == 0 {
					wx, wy = 7.3, 2.0
				} else {
					wx, wy = 8.3, 0.7
				}
			}
		} else {
			if sequence < 5 {
				wx = 6.15 - float32(sequence)*1.1
				wy = -2.6
			} else if sequence == 5 {
				if rule == 0 {
					wx, wy = 7.3, -0.7
				} else {
					wx, wy = 7.3, -2.0
				}
			} else if sequence == 6 {
				if rule == 0 {
					wx, wy = 7.3, -2.0
				} else {
					wx, wy = 7.3, -0.7
				}
			} else {
				if rule == 0 {
					wx, wy = 0.6, -2.0
				} else {
					wx, wy = -0.4, -0.7
				}
			}
		}
	case 0x10: // GRAVE
		if controler == 0 {
			if rule == 0 {
				wx, wy = 7.3, 0.7
			} else {
				wx, wy = 7.3, 2.0
			}
		} else {
			if rule == 0 {
				wx, wy = 0.6, -0.7
			} else {
				wx, wy = 0.6, -2.0
			}
		}
	case 0x20: // REMOVED
		if controler == 0 {
			if rule == 0 {
				wx, wy = 8.3, 0.7
			} else {
				wx, wy = 7.3, 0.7
			}
		} else {
			if rule == 0 {
				wx, wy = -0.4, -0.7
			} else {
				wx, wy = 0.6, -0.7
			}
		}
	case 0x40: // EXTRA
		if controler == 0 {
			wx, wy = 0.6, 3.3
		} else {
			wx, wy = 7.3, -3.3
		}
	case 0x80: // OVERLAY
		// overlay target must be in MZONE
		return 0, 0
	}
	return worldToScreen(wx, wy)
}

// GetCardLocation computes the target screen position and rotation for a card.
// Translated from C++ ClientField::GetCardLocation.
func (cf *ClientField) GetCardLocation(pcard *ClientCard) (tx, ty, rotZ float32, faceUp bool) {
	controler := int(pcard.Controler)
	sequence := int(pcard.Sequence)
	location := pcard.Location
	rule := 0
	if MainGame.DInfo.DuelRule >= 4 {
		rule = 1
	}

	tx, ty = zoneCenter(controler, location, sequence, rule)

	// Stack offset for deck/grave/remove/extra
	if location == 0x01 || location == 0x10 || location == 0x20 || location == 0x40 {
		ty -= float32(sequence) * 2 // slight pixel stack offset
	}

	faceUp = true
	rotZ = 0

	switch location {
	case 0x01: // DECK
		if controler == 0 {
			if cf.DeckReversed == pcard.IsReversed {
				rotZ = float32(math.Pi)
				faceUp = false
			}
		} else {
			if cf.DeckReversed == pcard.IsReversed {
				rotZ = float32(math.Pi)
				faceUp = false
			} else {
				rotZ = float32(math.Pi)
			}
		}
	case 0x02: // HAND
		if controler == 0 {
			if pcard.Code == 0 {
				rotZ = float32(math.Pi)
				faceUp = false
			}
		} else {
			if pcard.Code == 0 {
				rotZ = float32(math.Pi)
				faceUp = false
			}
		}
	case 0x04: // MZONE
		if controler == 0 {
			if pcard.Position&0x04 != 0 { // DEFENSE
				rotZ = -float32(math.Pi) / 2.0
				if pcard.Position&0x08 != 0 { // FACEDOWN
					faceUp = false
				}
			} else {
				if pcard.Position&0x08 != 0 { // FACEDOWN ATTACK
					faceUp = false
				}
			}
		} else {
			if pcard.Position&0x04 != 0 { // DEFENSE
				rotZ = float32(math.Pi) / 2.0
				if pcard.Position&0x08 != 0 {
					faceUp = false
				}
			} else {
				rotZ = float32(math.Pi)
				if pcard.Position&0x08 != 0 {
					faceUp = false
				}
			}
		}
	case 0x08: // SZONE
		if controler == 0 {
			if pcard.Position&0x08 != 0 {
				faceUp = false
			}
		} else {
			rotZ = float32(math.Pi)
			if pcard.Position&0x08 != 0 {
				faceUp = false
			}
		}
	case 0x10: // GRAVE
		if controler == 1 {
			rotZ = float32(math.Pi)
		}
	case 0x20: // REMOVED
		if controler == 0 {
			if pcard.Position&0x08 != 0 {
				faceUp = false
			}
		} else {
			rotZ = float32(math.Pi)
			if pcard.Position&0x08 != 0 {
				faceUp = false
			}
		}
	case 0x40: // EXTRA
		if controler == 0 {
			if pcard.Position&0x05 != 0 { // face-up
				// rotZ = 0
			} else {
				faceUp = false
				rotZ = float32(math.Pi)
			}
		} else {
			rotZ = float32(math.Pi)
			if pcard.Position&0x05 != 0 {
				// face-up
			} else {
				faceUp = false
			}
		}
	case 0x80: // OVERLAY
		if pcard.OverlayTarget != nil && pcard.OverlayTarget.Location == 0x04 {
			oseq := int(pcard.OverlayTarget.Sequence)
			mseq := sequence
			if mseq > 4 {
				mseq = 4
			}
			tx, ty = zoneCenter(controler, 0x04, oseq, rule)
			if controler == 0 {
				tx -= 12 - float32(mseq)*6
				ty += 4
			} else {
				tx += 12 - float32(mseq)*6
				ty -= 4
			}
		}
	}
	return
}
