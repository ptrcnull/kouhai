package ui

import (
	"hash/fnv"
	"math"
	"strings"

	"git.sr.ht/~rockorager/vaxis"
)

var ColorDefault = vaxis.Color(0)
var ColorGreen = vaxis.IndexColor(2)
var ColorRed = vaxis.IndexColor(9)
var ColorGray = vaxis.IndexColor(8)

type ColorSchemeType int

type ColorScheme struct {
	Type   ColorSchemeType
	Others vaxis.Color
	Self   vaxis.Color
}

const (
	ColorSchemeBase ColorSchemeType = iota
	ColorSchemeExtended
	ColorSchemeFixed
)

var baseColors = []vaxis.Color{
	// base 16 colors, excluding grayscale colors.
	vaxis.IndexColor(1),
	vaxis.IndexColor(2),
	vaxis.IndexColor(3),
	vaxis.IndexColor(4),
	vaxis.IndexColor(5),
	vaxis.IndexColor(6),
	vaxis.IndexColor(7),
	vaxis.IndexColor(9),
	vaxis.IndexColor(10),
	vaxis.IndexColor(11),
	vaxis.IndexColor(12),
	vaxis.IndexColor(13),
	vaxis.IndexColor(14),
}

func hslToRGB(hue, sat, light float64) (r, g, b uint8) {
	var r1, g1, b1 float64
	chroma := (1 - math.Abs(2*light-1)) * sat
	h6 := hue / 60
	x := chroma * (1 - math.Abs(math.Mod(h6, 2)-1))
	if h6 < 1 {
		r1, g1, b1 = chroma, x, 0
	} else if h6 < 2 {
		r1, g1, b1 = x, chroma, 0
	} else if h6 < 3 {
		r1, g1, b1 = 0, chroma, x
	} else if h6 < 4 {
		r1, g1, b1 = 0, x, chroma
	} else if h6 < 5 {
		r1, g1, b1 = x, 0, chroma
	} else {
		r1, g1, b1 = chroma, 0, x
	}
	m := light - chroma/2
	r = uint8(math.MaxUint8 * (r1 + m))
	g = uint8(math.MaxUint8 * (g1 + m))
	b = uint8(math.MaxUint8 * (b1 + m))
	return r, g, b
}

func (ui *UI) IdentColor(scheme ColorScheme, ident string, self bool) vaxis.Color {
	if self && scheme.Self != 0 {
		return scheme.Self
	}
	h := fnv.New32()
	_, _ = h.Write([]byte(ident))
	switch scheme.Type {
	case ColorSchemeFixed:
		if self {
			return ColorRed
		} else {
			return scheme.Others
		}
	case ColorSchemeBase:
		return baseColors[int(h.Sum32()%uint32(len(baseColors)))]
	case ColorSchemeExtended:
		sum := h.Sum32()
		lo := int(sum & 0xFFFF)
		hi := int((sum >> 16) & 0xFFFF)
		hue := float64(lo) / float64(math.MaxUint16) * 360
		sat := 1.0
		var lightMin, lightMax float64
		switch ui.colorThemeMode {
		case vaxis.DarkMode:
			lightMin, lightMax = 0.5, 0.7
		case vaxis.LightMode:
			lightMin, lightMax = 0.2, 0.4
		}
		light := lightMin + float64(hi)/float64(math.MaxUint16)*(lightMax-lightMin)
		return vaxis.RGBColor(hslToRGB(hue, sat, light))
	default:
		panic("invalid color scheme setting")
	}
	baseName := strings.ToLower(ident)
	var angleBase uint64 = 0
	angleBase += uint64(CapLetter(baseName[0])) * 28
	if len(baseName) > 1 {
		angleBase += uint64(CapLetter(baseName[1]))
	}
	// full spectrum
	var maxValues float64 = 27 * 28
	// make it rotate thrice
	maxValues /= 3

	_, angle := math.Modf(float64(angleBase) / maxValues)
	// 360 no scope
	hue := angle * 360

	return vaxis.RGBColor(HSVToRGB(hue, 1, 1))
}

// returns a value between 0 and 27 for a given character
func CapLetter(value byte) byte {
	if value < 'a' {
		value = ('a' - 1)
	}
	if value > 'z' {
		value = ('z' + 1)
	}
	value -= ('a' - 1)
	return value
}

func (ui *UI) IdentString(scheme ColorScheme, ident string, self bool) StyledString {
	color := ui.IdentColor(scheme, ident, self)
	style := vaxis.Style{
		Foreground: color,
	}
	return Styled(ident, style)
}

// HSVToRGB converts an HSV triple to an RGB triple.
func HSVToRGB(h, s, v float64) (r, g, b uint8) {
	if h < 0 || h >= 360 ||
		s < 0 || s > 1 ||
		v < 0 || v > 1 {
		return 0, 0, 0
	}
	// When 0 ≤ h < 360, 0 ≤ s ≤ 1 and 0 ≤ v ≤ 1:
	C := v * s
	X := C * (1 - math.Abs(math.Mod(h/60, 2)-1))
	m := v - C
	var Rnot, Gnot, Bnot float64
	switch {
	case 0 <= h && h < 60:
		Rnot, Gnot, Bnot = C, X, 0
	case 60 <= h && h < 120:
		Rnot, Gnot, Bnot = X, C, 0
	case 120 <= h && h < 180:
		Rnot, Gnot, Bnot = 0, C, X
	case 180 <= h && h < 240:
		Rnot, Gnot, Bnot = 0, X, C
	case 240 <= h && h < 300:
		Rnot, Gnot, Bnot = X, 0, C
	case 300 <= h && h < 360:
		Rnot, Gnot, Bnot = C, 0, X
	}
	r = uint8(math.Round((Rnot + m) * 255))
	g = uint8(math.Round((Gnot + m) * 255))
	b = uint8(math.Round((Bnot + m) * 255))
	return r, g, b
}
