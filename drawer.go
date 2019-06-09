package text2img

import (
	// "errors"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/jpeg"
	_ "image/png"
	"io/ioutil"
	"math"
	"os"
	"strconv"
	"strings"

	"github.com/golang/freetype"
	"github.com/golang/freetype/truetype"
	"golang.org/x/image/font"
	"golang.org/x/image/font/basicfont"
)

// Drawer is the main interface for this package
type Drawer interface {
	Draw(string)
	SetColors(color.RGBA, color.RGBA)
	SetFontPath(string) error
	SetFontSize(float64)
	SetTextPos(int, int)
	SetSize(int, int)
}

// Params is parameters for NewDrawer function
type Params struct {
	Width               int
	Height              int
	FontPath            string
	BackgroundImagePath string
	FontSize            float64
	BackgroundColor     color.RGBA
	TextColor           color.RGBA
	TextPosVertical     int
	TextPosHorizontal   int
	NotesSource			string
	OutputFolder		string
}

// NewDrawer returns Drawer interface
func NewDrawer(params Params) (Drawer, error) {
	d := &drawer{}
	
	d.SetNotesSource(params.NotesSource)
	d.SetOutputFolder(params.OutputFolder)

	if params.FontPath != "" {
		err := d.SetFontPath(params.FontPath)
		if err != nil {
			return d, err
		}
	}
	if params.BackgroundImagePath != "" {
		err := d.SetBackgroundImage(params.BackgroundImagePath)
		if err != nil {
			return d, err
		}
		d.SetSize(d.BackgroundImage.Bounds().Size().X, d.BackgroundImage.Bounds().Size().Y)
	} else {
		d.SetSize(params.Width, params.Height)
	}

	// d.SetColors(params.TextColor, params.BackgroundColor)
	// d.SetFontSize(params.FontSize)

	return d, nil
}

type drawer struct {
	BackgroundColor   *image.Uniform
	BackgroundImage   image.Image
	Font              *truetype.Font
	FontSize          float64
	Height            int
	TextColor         *image.Uniform
	TextPosVertical   int
	TextPosHorizontal int
	Width             int
	NotesSource 	  string
	OutputFolder	  string

	autoFontSize bool
}

// type NotesType int

// const (
// 	Text 	NotesType = 0
// 	Code 	NotesType = 1
// )

// type Separator int

// const (
// 	Dot		Separator = 0
// 	Code	Separator = 1
// )

/*
	Set the notes type to be code/text.
	Detect snippets b/w ------ which are code snippets - multiple lines of code that should be displayed in one image.
	Detect snippets b/w ++++++ which are text snippets - multiple lines of text that should be displayed in one image.
	Everything else, which is neither b/w ------, nor b/w ++++++ are text snippets as well once split into lines.

	type CodeSentence string
	type TextSentence string
	type TextLine	  string

	func (ts *TextSentence) ToTextLines() ([]TextLine)

	type CodeSnippet []CodeSentence
	type TextSnippet []TextSentence

	[]CodeSnippet
	[]TextSnippet
*/

func choose(ss []string, test func(string) bool) (ret []string) {
    for _, s := range ss {
        if test(s) {
            ret = append(ret, s)
        }
    }
    return
}

func WordsLongerThan3Letters(str string) ([]string) {
	words := strings.Split(str, " ")
	//ignore 1 and 2 letter words
	func_words_with_more_than_2_letters := func(s string) bool { return len(s) > 2 }
	words_with_more_than_2_letters := choose(words, func_words_with_more_than_2_letters)	

	return words_with_more_than_2_letters
}

func Words(str string) ([]string) {
	words := strings.Split(str, " ")

	return words
}

func (d *drawer) Snippets(text string) ([][]string) {
	// chunks := strings.Split(text + " ", ". ")
	// return chunks
	//TODO: Convert into enum

	snippets := make([][]string, 0)

	lines := strings.Split(text, "\n")

	accumulateCodeSnippet := false
	accumulateTextSnippet := false
	codeSnippet := make([]string, 0)
	textSnippet := make([]string, 0)

	codeSnippetStart := "{{{{{{"
	codeSnippetEnd := "}}}}}}"
	textSnippetStart := "[[[[[["
	textSnippetEnd := "]]]]]]"
	if d.NotesSource == "liner" {
		textSnippetStart = "___"
		textSnippetEnd = "___"		
	}

	for _, line := range lines {
		line = strings.Trim(line, " \t\n\r")
		if line == "" {
			continue
		}

		if line == codeSnippetStart {
			accumulateCodeSnippet = true
			continue
		}

		if line == codeSnippetEnd {
			accumulateCodeSnippet = false
			snippets = append(snippets, codeSnippet)
			codeSnippet = make([]string, 0)
			continue
		}

		if accumulateCodeSnippet {
			codeSnippet = append(codeSnippet, line)
			continue
		}

		if line == textSnippetStart {
			if d.NotesSource == "liner" {
				if accumulateTextSnippet == true {
					//Handle Text Snippet End here itself!
					accumulateTextSnippet = false
					snippets = append(snippets, textSnippet)
					textSnippet = make([]string, 0)
				}
			}
			accumulateTextSnippet = true
			continue
		}

		if line == textSnippetEnd {
			accumulateTextSnippet = false
			snippets = append(snippets, textSnippet)
			textSnippet = make([]string, 0)
			continue
		}

		//The line we are scanning is either part of "Text Snippet Accumulation", or "Single Line Text"

		if line[len(line) - 1] != '.' {
			line = line + ". "
		} else {
			line = line + " "
		}

		sentences := strings.Split(line, ". ")
		for _, sentence := range sentences {
			if sentence == "" {
				continue
			}
			sentence = sentence + "."

			// words := strings.Split(sentence, " ")
			// //ignore 1 and 2 letter words
			// func_words_with_more_than_2_letters := func(s string) bool { return len(s) > 2 }
			words_with_more_than_2_letters := WordsLongerThan3Letters(sentence)
			if (len(words_with_more_than_2_letters) > 12) {
				//split this sentence further. Try splitting by commas
				phrases := strings.Split(sentence, ",")
				for index, phrase := range phrases {
					if phrase == "" {
						continue
					}
					if index == len(phrases) - 1 && sentence[len(sentence) - 1] == ',' {
						phrase = phrase + ","
					}

					words := Words(phrase)
					words_with_more_than_2_letters := WordsLongerThan3Letters(phrase)
					// words = strings.Split(phrase, " ")
					// words_with_more_than_2_letters := choose(words, func_words_with_more_than_2_letters)
					if (len(words_with_more_than_2_letters) > 12) {
						phraseParts := 2;
						for len(words) / phraseParts > 12 {
							phraseParts = phraseParts + 1
						}
						wordsInOnePhrasePart := len(words) / phraseParts
						for len(words) > 0 {
							upperBoundIndex := wordsInOnePhrasePart
							if wordsInOnePhrasePart > len(words) {
								upperBoundIndex = len(words)
							}
							phrasePart := strings.Join(words[0 : upperBoundIndex], " ")

							textSnippet = append(textSnippet, phrasePart)

							words = words[upperBoundIndex:]
						}
					} else {
						if len(textSnippet) > 0 && (len(WordsLongerThan3Letters(textSnippet[len(textSnippet) - 1])) + len(WordsLongerThan3Letters(phrase))) <=13 {
							textSnippet[len(textSnippet) - 1] = textSnippet[len(textSnippet) - 1] + ", " + phrase
						} else {
							textSnippet = append(textSnippet, phrase)
						}
					}
				}
			} else {
				if len(textSnippet) > 0 && (len(WordsLongerThan3Letters(textSnippet[len(textSnippet) - 1])) + len(WordsLongerThan3Letters(sentence))) <=13 {
					textSnippet[len(textSnippet) - 1] = textSnippet[len(textSnippet) - 1] + " " + sentence
				} else {
					textSnippet = append(textSnippet, sentence)
				}
			}
		}

		if accumulateTextSnippet == false {
			snippets = append(snippets, textSnippet)
			textSnippet = make([]string, 0)
		}
		
		// //Odd chunk!
		// if index % 2 == 1 {
		// 	chunkLines = strings.Split(chunk, "\n")
		// }
	}

	for _, snippet := range snippets {
		for _, line := range snippet {
			fmt.Printf("Snippet Line: %s\n", line)
		}
	}

	return snippets
}

// Draw returns the image of a text
func (d *drawer) Draw(text string) {

	snippets := d.Snippets(text)

	// if d.autoFontSize {
	// 	d.FontSize = 10000
	// 	for _, snippet := range snippets {
	// 		for _, line := range snippet {
	// 			d.FontSize = math.Min(d.FontSize, d.calcFontSize(line))
	// 		}
	// 	}
	// }

	for index, snippet := range snippets {
		var bgColor, textColor color.RGBA

		d.SetColors(bgColor, textColor)
		//let it use auto font size
		d.SetFontSize(0)

		strIndex := strconv.Itoa(index)

		if len(snippet) > 0 && strings.HasPrefix(snippet[0], "PLACEHOLDER_IMAGE ") {
			placeholderFilename := strings.Replace(snippet[0], "PLACEHOLDER_IMAGE ", "", -1)
			fmt.Printf("PLACEHOLDER FILE NAME = %s\n", placeholderFilename)
			os.Rename(d.OutputFolder + "\\" + placeholderFilename, d.OutputFolder + "\\" + strIndex + ".jpg")
		} else {
			d.drawSnippet(snippet, d.OutputFolder + "\\" + strIndex + ".jpg")
		}
	}
}

func (d *drawer) drawSnippet(lines []string, output string) {
	var img *image.RGBA
	var err error

	if d.autoFontSize {
		d.FontSize = 10000
		for _, line := range lines {
			d.FontSize = math.Min(d.FontSize, d.calcFontSize(line))
		}
	}


	if d.BackgroundImage != nil {
		imgRect := image.Rectangle{image.Pt(0, 0), d.BackgroundImage.Bounds().Size()}
		img = image.NewRGBA(imgRect)
		draw.Draw(img, img.Bounds(), d.BackgroundImage, image.ZP, draw.Src)
	} else {
		img = image.NewRGBA(image.Rect(0, 0, d.Width, d.Height))
		draw.Draw(img, img.Bounds(), d.BackgroundColor, image.ZP, draw.Src)
	}

	// var lines [3]string
	// lines[0] = "$ yarn global add create-react-app"
	// lines[1] = "$ create-react-app react-hello"
	// lines[2] = "$ rm src/App.* src/index.css src/logo.svg"
	
	if d.Font != nil {
		c := freetype.NewContext()
		c.SetDPI(72)
		c.SetFont(d.Font)
		c.SetFontSize(d.FontSize)
		c.SetClip(img.Bounds())
		c.SetDst(img)
		c.SetSrc(d.TextColor)
		c.SetHinting(font.HintingNone)

		gapFromLastLine := 0
		textHeight := int(c.PointToFixed(d.FontSize) >> 6)
		startingHeightPoint := (d.Height - len(lines) * textHeight - (len(lines) - 1) * 40) / 2

		for _, line := range lines {
			line = strings.Trim(line, " \t\n\r")
			if line == "" {
				continue
			}
			// line = line + "."
			textWidth := d.calcTextWidth(d.FontSize, line)
			// pt := freetype.Pt((d.Width-textWidth)/2+d.TextPosHorizontal, (d.Height+textHeight)/2+d.TextPosVertical + gapFromLastLine)
			pt := freetype.Pt((d.Width-textWidth)/2+d.TextPosHorizontal, startingHeightPoint + gapFromLastLine)
			gapFromLastLine += textHeight + 40
			_, err = c.DrawString(line, pt)
		}

		//return
	}
	//err = errors.New("Font must be specified")
	// point := fixed.Point26_6{640, 960}
	// fd := &font.Drawer{
	// 	Dst:  img,
	// 	Src:  d.TextColor,
	// 	Face: basicfont.Face7x13,
	// 	Dot:  point,
	// }
	// fd.DrawString(text)

	file, err := os.Create(output)
	if err != nil {
		panic(err.Error())
	}

	defer file.Close()

	if err = jpeg.Encode(file, img, &jpeg.Options{Quality: 100}); err != nil {
		panic(err.Error())
	}
}

// SetBackgroundImage sets the specific background image
func (d *drawer) SetBackgroundImage(imagePath string) (err error) {
	src, err := os.Open(imagePath)
	if err != nil {
		return
	}
	defer src.Close()

	img, _, err := image.Decode(src)
	if err != nil {
		return
	}
	d.BackgroundImage = img
	return
}

// SetColors sets the textColor and the backgroundColor
func (d *drawer) SetColors(textColor, backgroundColor color.RGBA) {
	r1, g1, b1, a1 := backgroundColor.RGBA()
	r2, g2, b2, a2 := textColor.RGBA()
	if r1 == r2 && g1 == g2 && b1 == b2 && a1 == a2 {
		color := PickColor()
		d.TextColor = image.NewUniform(color.TextColor)
		d.BackgroundColor = image.NewUniform(color.BackgroundColor)
		return
	}
	d.TextColor = image.NewUniform(textColor)
	d.BackgroundColor = image.NewUniform(backgroundColor)
}

// SetColors sets the font
func (d *drawer) SetFontPath(fontPath string) (err error) {
	fontBytes, err := ioutil.ReadFile(fontPath)
	if err != nil {
		return
	}
	f, err := freetype.ParseFont(fontBytes)
	if err != nil {
		return
	}
	d.Font = f
	return
}

// SetColors sets the fontSize
func (d *drawer) SetFontSize(fontSize float64) {
	if fontSize > 0 {
		d.autoFontSize = false
		d.FontSize = fontSize
		return
	}
	d.autoFontSize = true
}

// SetFontPos sets the fontPos
func (d *drawer) SetTextPos(textPosVertical, textPosHorizontal int) {
	d.TextPosVertical = textPosVertical
	d.TextPosHorizontal = textPosHorizontal
}

// SetColors sets the size
func (d *drawer) SetSize(width, height int) {
	if width <= 0 {
		d.Width = 1200
	} else {
		d.Width = width
	}
	if height <= 0 {
		d.Height = 630
	} else {
		d.Height = height
	}
}

func (d *drawer) SetNotesSource(notesSource string) {
	d.NotesSource = notesSource
}

func (d *drawer) SetOutputFolder(outputFolder string) {
	d.OutputFolder = outputFolder
}

func (d *drawer) calcFontSize(text string) (fontSize float64) {
	const padding = 4
	fontSizes := []float64{128, 64, 48, 32, 24, 18, 16, 14, 12}
	for _, fontSize = range fontSizes {
		textWidth := d.calcTextWidth(fontSize, text)
		if textWidth < d.Width {
			return
		}
	}
	return
}

func (d *drawer) calcTextWidth(fontSize float64, text string) (textWidth int) {
	var face font.Face
	if d.Font != nil {
		opts := truetype.Options{}
		opts.Size = fontSize
		face = truetype.NewFace(d.Font, &opts)
	} else {
		face = basicfont.Face7x13
	}
	for _, x := range text {
		awidth, ok := face.GlyphAdvance(rune(x))
		if ok != true {
			return
		}
		textWidth += int(float64(awidth) / 64)
	}
	return
}
