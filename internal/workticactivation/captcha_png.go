package workticactivation

import (
	"bytes"
	"errors"
	"image"
	"image/color"
	"image/png"
	"math"
)

func renderSymbolPNG(kind string) ([]byte, error) {
	paths := map[string][][][2]int{
		"chat":     {{{20, 18}, {68, 18}, {68, 50}, {42, 50}, {26, 61}, {29, 50}, {20, 50}, {20, 18}}, {{31, 30}, {56, 30}}, {{31, 39}, {50, 39}}},
		"spark":    {{{44, 8}, {52, 28}, {73, 36}, {52, 44}, {44, 64}, {36, 44}, {15, 36}, {36, 28}, {44, 8}}},
		"shield":   {{{44, 9}, {66, 20}, {65, 43}, {57, 53}, {44, 64}, {31, 53}, {23, 43}, {22, 20}, {44, 9}}, {{32, 36}, {41, 44}, {56, 28}}},
		"calendar": {{{21, 21}, {67, 21}, {67, 59}, {21, 59}, {21, 21}}, {{21, 31}, {67, 31}}, {{33, 15}, {33, 26}}, {{55, 15}, {55, 26}}},
		"bolt":     {{{47, 8}, {25, 40}, {42, 40}, {38, 64}, {65, 29}, {47, 29}, {47, 8}}},
	}

	if kind == "heart" {
		points := make([][2]int, 0, 41)
		for i := 0; i <= 40; i++ {
			t := 2 * math.Pi * float64(i) / 40
			x := 44 + int(math.Round(1.35*16*math.Pow(math.Sin(t), 3)))
			y := 30 - int(math.Round(1.1*(13*math.Cos(t)-5*math.Cos(2*t)-2*math.Cos(3*t)-math.Cos(4*t))))
			points = append(points, [2]int{x, y})
		}
		paths[kind] = [][][2]int{points}
	}

	segments, ok := paths[kind]
	if !ok {
		return nil, errors.New("unknown captcha symbol")
	}

	img := image.NewNRGBA(image.Rect(0, 0, 88, 72))
	colors := []color.NRGBA{
		{43, 118, 250, 255},
		{141, 65, 246, 255},
		{13, 193, 221, 255},
	}

	ci, err := secureIndex(len(colors))
	if err != nil {
		return nil, err
	}
	sx, err := secureIndex(7)
	if err != nil {
		return nil, err
	}
	sy, err := secureIndex(7)
	if err != nil {
		return nil, err
	}

	dot := func(x, y int) {
		for dx := -2; dx <= 2; dx++ {
			for dy := -2; dy <= 2; dy++ {
				if dx*dx+dy*dy <= 4 && x+dx >= 0 && y+dy >= 0 && x+dx < 88 && y+dy < 72 {
					img.SetNRGBA(x+dx, y+dy, colors[ci])
				}
			}
		}
	}

	for _, segment := range segments {
		for i := 1; i < len(segment); i++ {
			x0, y0 := segment[i-1][0], segment[i-1][1]
			x1, y1 := segment[i][0], segment[i][1]
			steps := int(math.Max(math.Abs(float64(x1-x0)), math.Abs(float64(y1-y0))))*2 + 1
			for n := 0; n <= steps; n++ {
				t := float64(n) / float64(steps)
				x := int(math.Round(float64(x0)+float64(x1-x0)*t)) + sx - 3
				y := int(math.Round(float64(y0)+float64(y1-y0)*t)) + sy - 3
				dot(x, y)
			}
		}
	}

	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}
