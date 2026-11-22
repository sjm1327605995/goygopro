package tenon

import (
	"strconv"
	"sync"

	"github.com/g3n/engine/geometry"
	"github.com/g3n/engine/math32"
	"github.com/g3n/engine/texture"
	"github.com/sjm1327605995/tenon/backend/g3n"
	"github.com/sjm1327605995/tenon/pkg/value"
)

func rgb8(r, g, b uint8) g3n.Color {
	return g3n.RGB(float32(r)/255, float32(g)/255, float32(b)/255)
}

var arenaStoneTexture = sync.OnceValue(func() *texture.Texture2D {
	return g3n.TextureFromImage(arenaStoneImage())
})
var arenaGroundTexture = sync.OnceValue(func() *texture.Texture2D {
	return g3n.TextureFromImage(arenaGroundImage())
})

func newArenaScenery() ([]*g3n.MeshComponent, []boardZone) {
	meshes := []*g3n.MeshComponent{}
	zones := []boardZone{}
	add := func(mesh *g3n.MeshComponent) { meshes = append(meshes, mesh) }
	add(texturedMesh(g3n.GeometryShape(geometry.NewPlane(40, 32)),
		g3n.MaterialProps{Texture: arenaGroundTexture(), Unlit: true, CacheKey: "arena-ground"},
		g3n.V3(0, -1.2, 0), g3n.V3(-math32.Pi/2, 0, 0), g3n.V3(1, 1, 1)))
	add(nativeMesh(slabShape(22.2, 18, 0.62), rgb8(23, 35, 41), g3n.V3(0, -0.94, 0), g3n.V3(1, 1, 1)))
	add(nativeMesh(slabShape(21.6, 17.6, 0.15), rgb8(140, 121, 82), g3n.V3(0, -0.35, 0), g3n.V3(1, 1, 1)))
	add(nativeMesh(g3n.CubeShape(1), rgb8(37, 60, 59), g3n.V3(0, -0.15, 0), g3n.V3(20.4, 0.14, 16.4)))
	add(texturedMesh(g3n.GeometryShape(geometry.NewPlane(20, 16)),
		g3n.MaterialProps{Texture: arenaStoneTexture(), Unlit: true, CacheKey: "arena-stone"},
		g3n.V3(0, -0.075, 0), g3n.V3(-math32.Pi/2, 0, 0), g3n.V3(1, 1, 1)))
	stone, bronze, dark := rgb8(53, 68, 67), rgb8(140, 121, 82), rgb8(23, 35, 41)
	for _, side := range []float32{-1, 1} {
		for index := 0; index < 12; index++ {
			add(nativeMesh(g3n.CubeShape(1), stone, g3n.V3((float32(index)-5.5)*1.74, -0.12, side*8.55), g3n.V3(1.65, 0.36, 0.55)))
		}
		for index := 0; index < 9; index++ {
			add(nativeMesh(g3n.CubeShape(1), stone, g3n.V3(side*10.55, -0.12, (float32(index)-4)*1.82), g3n.V3(0.48, 0.34, 1.7)))
		}
	}
	seen := map[string]bool{}
	for seat := 0; seat < 2; seat++ {
		for _, location := range []uint8{locationMZone, locationSZone} {
			count := 8
			if location == locationMZone {
				count = 7
			}
			for sequence := 0; sequence < count; sequence++ {
				position := zonePosition(seat, location, sequence)
				key := pointKey(position)
				if seen[key] {
					continue
				}
				seen[key] = true
				color := rgb8(101, 122, 114)
				if seat == 1 {
					color = rgb8(122, 124, 96)
				}
				mesh := nativeMesh(slabShape(1.94, 2.16, 0.035), color, g3n.V3(position.X, -0.105, position.Z), g3n.V3(1, 1, 1))
				add(mesh)
				zones = append(zones, boardZone{hit: FieldHit{Seat: seat, Location: int(location), Sequence: sequence}, mesh: mesh})
			}
		}
		for _, location := range []uint8{locationGrave, locationBanish} {
			position := zonePosition(seat, location, 0)
			add(nativeMesh(g3n.GeometryShape(geometry.NewTruncatedCone(0.91, 0.96, 0.12, 36, 1, true, true)), dark, g3n.V3(position.X, -0.06, position.Z), g3n.V3(1, 1, 1)))
			add(g3n.Mesh(g3n.MeshProps{
				Shape: g3n.GeometryShape(geometry.NewTorus(0.79, 0.045, 6, 40, math32.Pi*2)), Material: g3n.StandardMaterial(value.Static(bronze)),
				Transform: g3n.TransformProps{Position: value.Static(ringPosition(position.X, position.Z)), Rotation: value.Static(g3n.V3(-math32.Pi/2, 0, 0)), Scale: value.Static(g3n.V3(1, 1, 1))},
			}))
		}
	}
	for _, x := range []float32{-10.45, 10.45} {
		for _, z := range []float32{-7.9, 7.9} {
			add(nativeMesh(slabShape(0.85, 0.85, 0.32), bronze, g3n.V3(x, -0.08, z), g3n.V3(1, 1, 1)))
			color := rgb8(127, 197, 186)
			if z < 0 {
				color = rgb8(210, 186, 121)
			}
			add(nativeMesh(octahedronShape(), color, g3n.V3(x, 0.64, z), g3n.V3(0.3, 0.51, 0.3)))
		}
	}
	for _, side := range []float32{-1, 1} {
		for _, z := range []float32{-5.1, 4.9} {
			add(nativeMesh(g3n.GeometryShape(geometry.NewTruncatedCone(0.28, 0.42, 1.35, 8, 1, true, true)), stone, g3n.V3(side*11.05, 0.15, z), g3n.V3(1, 1, 1)))
			add(nativeMesh(g3n.CubeShape(1), bronze, g3n.V3(side*11.05, 0.9, z), g3n.V3(0.9, 0.18, 0.9)))
			for index := 0; index < 3; index++ {
				add(nativeMesh(g3n.SphereShape(0.48, 8, 4), rgb8(52, 77, 54), g3n.V3(side*11.05, -0.05, z+float32(index-1)*0.62), g3n.V3(1.05, 0.5, 0.9)))
			}
		}
	}
	points := arenaOutlinePoints()
	add(g3n.Mesh(g3n.MeshProps{
		Shape:     arenaLinesShape(points),
		Material:  g3n.MaterialProps{Color: value.Static(rgb8(210, 194, 141)), Opacity: value.Static(float32(0.66)), Unlit: true, CacheKey: "arena-zone-lines"},
		Transform: g3n.TransformProps{Position: value.Static(g3n.V3(0, 0, 0))},
	}))
	return meshes, zones
}

func pointKey(point g3n.Vec3) string {
	return strconv.FormatInt(int64(math32.Round(point.X*100)), 10) + ":" + strconv.FormatInt(int64(math32.Round(point.Z*100)), 10)
}

func arenaOutlinePoints() []math32.Vector3 {
	corners := [][2]float32{{-0.68, -1.03}, {0.68, -1.03}, {0.9, -0.81}, {0.9, 0.81}, {0.68, 1.03}, {-0.68, 1.03}, {-0.9, 0.81}, {-0.9, -0.81}}
	seen := map[string]bool{}
	points := []math32.Vector3{}
	for seat := 0; seat < 2; seat++ {
		for _, location := range []uint8{locationMZone, locationSZone, locationDeck, locationExtra} {
			count := 1
			if location == locationMZone {
				count = 7
			} else if location == locationSZone {
				count = 8
			}
			for sequence := 0; sequence < count; sequence++ {
				position := zonePosition(seat, location, sequence)
				key := pointKey(position)
				if seen[key] {
					continue
				}
				seen[key] = true
				for i, corner := range corners {
					next := corners[(i+1)%len(corners)]
					points = append(points,
						math32.Vector3{X: position.X + corner[0], Y: 0.025, Z: position.Z + corner[1]},
						math32.Vector3{X: position.X + next[0], Y: 0.025, Z: position.Z + next[1]})
				}
			}
		}
	}
	return points
}
