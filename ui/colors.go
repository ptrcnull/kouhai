package ui

import (
	"hash/fnv"
	"math"
	"strings"

	"github.com/gdamore/tcell/v2"
)

type ColorSchemeType int

type ColorScheme struct {
	Type   ColorSchemeType
	Others tcell.Color
	Self   tcell.Color
}

const (
	ColorSchemeBase ColorSchemeType = iota
	ColorSchemeExtended
	ColorSchemeFixed
)

var colors = map[ColorSchemeType][]tcell.Color{
	// base 16 colors, excluding grayscale colors.
	ColorSchemeBase: {
		tcell.ColorMaroon,
		tcell.ColorGreen,
		tcell.ColorOlive,
		tcell.ColorNavy,
		tcell.ColorPurple,
		tcell.ColorTeal,
		tcell.ColorSilver,
		tcell.ColorRed,
		tcell.ColorLime,
		tcell.ColorYellow,
		tcell.ColorBlue,
		tcell.ColorFuchsia,
		tcell.ColorAqua,
	},
	// all XTerm extended colors with HSL saturation=1, light=0.5
	ColorSchemeExtended: {
		tcell.Color196, // HSL hue: 0°
		tcell.Color202, // HSL hue: 22°
		tcell.Color208, // HSL hue: 32°
		tcell.Color214, // HSL hue: 41°
		tcell.Color220, // HSL hue: 51°
		tcell.Color226, // HSL hue: 60°
		tcell.Color190, // HSL hue: 69°
		tcell.Color154, // HSL hue: 79°
		tcell.Color118, // HSL hue: 88°
		tcell.Color82,  // HSL hue: 98°
		tcell.Color46,  // HSL hue: 120°
		tcell.Color47,  // HSL hue: 142°
		tcell.Color48,  // HSL hue: 152°
		tcell.Color49,  // HSL hue: 161°
		tcell.Color50,  // HSL hue: 171°
		tcell.Color51,  // HSL hue: 180°
		tcell.Color45,  // HSL hue: 189°
		tcell.Color39,  // HSL hue: 199°
		tcell.Color33,  // HSL hue: 208°
		tcell.Color27,  // HSL hue: 218°
		tcell.Color21,  // HSL hue: 240°
		tcell.Color57,  // HSL hue: 262°
		tcell.Color93,  // HSL hue: 272°
		tcell.Color129, // HSL hue: 281°
		tcell.Color165, // HSL hue: 291°
		tcell.Color201, // HSL hue: 300°
		tcell.Color200, // HSL hue: 309°
		tcell.Color199, // HSL hue: 319°
		tcell.Color198, // HSL hue: 328°
		tcell.Color197, // HSL hue: 338°
	},
}

func IdentColor(scheme ColorScheme, ident string, self bool) tcell.Color {
	h := fnv.New32()
	_, _ = h.Write([]byte(ident))
	if scheme.Type == ColorSchemeFixed {
		if self {
			return scheme.Self
		} else {
			return scheme.Others
		}
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

	return tcell.NewRGBColor(HSVToRGB(hue, 1, 1))
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

func IdentString(scheme ColorScheme, ident string, self bool) StyledString {
	color := IdentColor(scheme, ident, self)
	style := tcell.StyleDefault.Foreground(color)
	return Styled(ident, style)
}

// HSVToRGB converts an HSV triple to an RGB triple.
func HSVToRGB(h, s, v float64) (r, g, b int32) {
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
	r = int32(math.Round((Rnot + m) * 255))
	g = int32(math.Round((Gnot + m) * 255))
	b = int32(math.Round((Bnot + m) * 255))
	return r, g, b
}
