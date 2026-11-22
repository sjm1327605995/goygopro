package tenon

import (
	"math"
	"sync"

	"github.com/g3n/engine/geometry"
	"github.com/g3n/engine/gls"
	"github.com/g3n/engine/math32"
	"github.com/sjm1327605995/tenon/backend/g3n"
	"github.com/sjm1327605995/tenon/pkg/value"
)

type triangleMeshBuilder struct {
	positions math32.ArrayF32
	normals   math32.ArrayF32
	uvs       math32.ArrayF32
}

func (b *triangleMeshBuilder) triangle(a, c, d math32.Vector3) {
	ab := c.Clone().Sub(&a)
	ac := d.Clone().Sub(&a)
	normal := ab.Cross(ac).Normalize()
	for _, point := range []math32.Vector3{a, c, d} {
		b.positions.Append(point.X, point.Y, point.Z)
		b.normals.Append(normal.X, normal.Y, normal.Z)
		b.uvs.Append(point.X+0.5, point.Z+0.5)
	}
}

func (b *triangleMeshBuilder) shape() g3n.Shape {
	geom := geometry.NewGeometry()
	geom.AddVBO(gls.NewVBO(b.positions).AddAttrib(gls.VertexPosition))
	geom.AddVBO(gls.NewVBO(b.normals).AddAttrib(gls.VertexNormal))
	geom.AddVBO(gls.NewVBO(b.uvs).AddAttrib(gls.VertexTexcoord))
	return g3n.GeometryShape(geom)
}

var slabShapes sync.Map

func slabShape(width, depth, height float32) g3n.Shape {
	key := [3]float32{width, depth, height}
	if cached, ok := slabShapes.Load(key); ok {
		return cached.(g3n.Shape)
	}
	cut := min(width, depth) * 0.14
	points := []math32.Vector3{
		{X: -width/2 + cut, Z: -depth / 2}, {X: width/2 - cut, Z: -depth / 2},
		{X: width / 2, Z: -depth/2 + cut}, {X: width / 2, Z: depth/2 - cut},
		{X: width/2 - cut, Z: depth / 2}, {X: -width/2 + cut, Z: depth / 2},
		{X: -width / 2, Z: depth/2 - cut}, {X: -width / 2, Z: -depth/2 + cut},
	}
	builder := triangleMeshBuilder{}
	for i, point := range points {
		next := points[(i+1)%len(points)]
		topA, topB := point, next
		topA.Y, topB.Y = height/2, height/2
		bottomA, bottomB := topA, topB
		bottomA.Y, bottomB.Y = -height/2, -height/2
		center := math32.Vector3{Y: height / 2}
		builder.triangle(center, topB, topA)
		builder.triangle(topA, topB, bottomB)
		builder.triangle(topA, bottomB, bottomA)
	}
	shape := builder.shape()
	slabShapes.Store(key, shape)
	return shape
}

func octahedronShape() g3n.Shape {
	points := []math32.Vector3{{Y: 1}, {Y: -1}, {X: 1}, {Z: 1}, {X: -1}, {Z: -1}}
	faces := [][3]int{{0, 2, 3}, {0, 3, 4}, {0, 4, 5}, {0, 5, 2}, {1, 3, 2}, {1, 4, 3}, {1, 5, 4}, {1, 2, 5}}
	builder := triangleMeshBuilder{}
	for _, face := range faces {
		builder.triangle(points[face[0]], points[face[1]], points[face[2]])
	}
	return builder.shape()
}

func nativeMesh(shape g3n.Shape, color g3n.Color, position g3n.Vec3, scale g3n.Vec3) *g3n.MeshComponent {
	return g3n.Mesh(g3n.MeshProps{
		Shape:     shape,
		Material:  g3n.StandardMaterial(value.Static(color)),
		Transform: g3n.TransformProps{Position: value.Static(position), Scale: value.Static(scale)},
	})
}

func texturedMesh(shape g3n.Shape, textureMaterial g3n.MaterialProps, position, rotation, scale g3n.Vec3) *g3n.MeshComponent {
	return g3n.Mesh(g3n.MeshProps{
		Shape: shape, Material: textureMaterial,
		Transform: g3n.TransformProps{Position: value.Static(position), Rotation: value.Static(rotation), Scale: value.Static(scale)},
	})
}

func arenaLinesShape(points []math32.Vector3) g3n.Shape {
	builder := triangleMeshBuilder{}
	for i := 0; i+1 < len(points); i += 2 {
		a, b := points[i], points[i+1]
		direction := b.Clone().Sub(&a)
		length := direction.Length()
		if length == 0 {
			continue
		}
		normal := math32.Vector3{X: -direction.Z, Z: direction.X}
		normal.Normalize().MultiplyScalar(0.012)
		aLeft, aRight := *a.Clone().Add(&normal), *a.Clone().Sub(&normal)
		bLeft, bRight := *b.Clone().Add(&normal), *b.Clone().Sub(&normal)
		builder.triangle(aLeft, bLeft, bRight)
		builder.triangle(aLeft, bRight, aRight)
	}
	return builder.shape()
}

func ringPosition(x, z float32) g3n.Vec3 { return g3n.V3(x, 0.015, z) }

func radians(value float64) float32 { return float32(value * math.Pi / 180) }
