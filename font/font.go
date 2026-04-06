package font

import (
	_ "embed"
	"fmt"

	"gioui.org/font"
	"gioui.org/font/opentype"
)

//go:embed JetBrainsMono-Regular.ttf
var jetbrainsMonoRegular []byte

//go:embed JetBrainsMono-Bold.ttf
var jetbrainsMonoBold []byte

func Collection() []font.FontFace {
	var collection []font.FontFace
	for _, ttf := range [][]byte{jetbrainsMonoRegular, jetbrainsMonoBold} {
		faces, err := opentype.ParseCollection(ttf)
		if err != nil {
			panic(fmt.Errorf("failed to parse font: %v", err))
		}
		collection = append(collection, faces[0])
	}
	return collection
}
