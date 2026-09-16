package display

import (
	"image"
	"image/color"
	"strings"

	"github.com/z3le/alien-buddy/content"
	"golang.org/x/image/font"
	"golang.org/x/image/font/inconsolata"
	"golang.org/x/image/math/fixed"
)

// Renderer draws aliens + text onto a 264×176 black-and-white image.
type Renderer struct {
	face font.Face
}

func NewRenderer() *Renderer {
	return &Renderer{
		face: inconsolata.Regular8x16,
	}
}

// Render creates a display-sized image with an alien (chosen by mood)
// and a text message below it.
func (r *Renderer) Render(mood content.Mood, text string) image.Image {
	img := image.NewGray(image.Rect(0, 0, Width, Height))

	// Fill white
	for y := 0; y < Height; y++ {
		for x := 0; x < Width; x++ {
			img.SetGray(x, y, color.Gray{Y: 255})
		}
	}

	alien := alienForMood(mood)
	alienLines := strings.Split(alien, "\n")

	// Draw alien starting near the top, centered horizontally
	y := 12
	for _, line := range alienLines {
		if line == "" {
			continue
		}
		xPos := centerX(line, r.face)
		r.drawString(img, xPos, y, line)
		y += 18
	}

	// Add a divider line
	y += 5
	for x := 20; x < Width-20; x++ {
		img.SetGray(x, y, color.Gray{Y: 0})
	}
	y += 12

	// Word-wrap and draw the text
	wrapped := wordWrap(text, Width-20, r.face)
	for _, line := range wrapped {
		xPos := centerX(line, r.face)
		r.drawString(img, xPos, y, line)
		y += 18

		if y > Height-10 {
			break // don't overflow the display
		}
	}

	return img
}

func (r *Renderer) drawString(img *image.Gray, x, y int, s string) {
	d := &font.Drawer{
		Dst:  img,
		Src:  image.NewUniform(color.Black),
		Face: r.face,
		Dot:  fixed.P(x, y),
	}
	d.DrawString(s)
}

// centerX returns the x position to center a string on the display.
func centerX(s string, face font.Face) int {
	adv := font.MeasureString(face, s)
	textW := adv.Ceil()
	x := (Width - textW) / 2
	if x < 4 {
		x = 4
	}
	return x
}

// wordWrap breaks text into lines that fit within maxWidth pixels.
func wordWrap(text string, maxWidth int, face font.Face) []string {
	words := strings.Fields(text)
	if len(words) == 0 {
		return nil
	}

	var lines []string
	current := words[0]

	for _, word := range words[1:] {
		test := current + " " + word
		adv := font.MeasureString(face, test)
		if adv.Ceil() > maxWidth {
			lines = append(lines, current)
			current = word
		} else {
			current = test
		}
	}
	lines = append(lines, current)
	return lines
}

// alienForMood returns ASCII art for the given mood.
// Falls back to "happy" for unknown moods.
func alienForMood(mood content.Mood) string {
	art, ok := aliens[mood]
	if !ok {
		art = aliens[content.MoodHappy]
	}
	return art
}

var aliens = map[content.Mood]string{
	content.MoodHappy: `
     .  *  .
   . _\|/_ .
    ( ^_^ )
    --{     }--
    /_   _\
   (_/   \_)
`,
	content.MoodStern: `
     .  *  .
   . _\|/_ .
    ( -_- )
    --{     }--
    /_   _\
   (_/   \_)
`,
	content.MoodConfused: `
     .  *  .
   . _\|/_ .
    ( o_O )
    --{     }--
    /_   _\
   (_/   \_)
`,
	content.MoodSleepy: `
     .  *  .
   . _\|/_ .
    ( u.u )
    --{  ~  }--
    /_   _\
   (_/   \_)
`,
	content.MoodExcited: `
     . *** .
   . _\|/_ .
    ( ^o^ )
    \-{     }-/
    /_   _\
   (_/   \_)
`,
	content.MoodThinking: `
        ?
     .  *  .
   . _\|/_ .
    ( ~_~ )
    --{     }--
    /_   _\
   (_/   \_)
`,
}
