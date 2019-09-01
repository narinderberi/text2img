package text2img

import (
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/jpeg"
	_ "image/png"
	"io/ioutil"
	"math"
	"os"
	"path/filepath"
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

func WordsLongerThan2LettersIn(str string) ([]string) {
	words := strings.Split(str, " ")
	//ignore 1 and 2 letter words
	func_words_with_more_than_2_letters := func(str string) bool { return len(str) > 2 }
	words_with_more_than_2_letters := choose(words, func_words_with_more_than_2_letters)	

	return words_with_more_than_2_letters
}

func nonEmptyStringChooser(str string) bool {
	return len(str) > 0
}

func Words(str string) ([]string) {
	words := strings.Split(str, " ")
	return choose(words, nonEmptyStringChooser)
}

func Lines(str string) ([]string) {
	lines := make([]string, 0)
	for _, line := range strings.Split(str, "\n") {
		line = strings.Trim(line, " \t\n\r")
		if line != "" {
			lines = append(lines, line)
		}
	}
	return choose(lines, nonEmptyStringChooser)
}

func Sentences(str string) ([] string) {
	sentences := make([]string, 0)
	// Replace ... with ___ as a simple way to support vocabulary blank sentences.
	str = strings.Replace(str, "...", "___", -1)
	splitByDot := strings.Split(str, ". ")

	for index, sentence := range splitByDot {
		sentence = strings.Trim(sentence, " \t\n\r")
		if sentence != "" {
			if index < len(splitByDot) - 1 {
				sentence = sentence + "."
			} else {
				strTrimmed := strings.Trim(str, " \t\n\r")
				if LastChar(strTrimmed) == "." {
					sentence = sentence + "."
				}
			}
			// Restore the ... if any which were replaced by ___ above.
			sentence = strings.Replace(sentence, "___", "...", -1)
			sentences = append(sentences, sentence)
		}
	}
	return choose(sentences, nonEmptyStringChooser)
}

func Phrases(str string) ([] string) {
	phrases := make([]string, 0)
	for _, phrase := range strings.Split(str, ",") {
		// phrase = strings.Trim(phrase, " \t\n\r")
		if strings.Trim(phrase, " \t\n\r") != "" {
			phrases = append(phrases, phrase)
		}
	}
	return choose(phrases, nonEmptyStringChooser)
}

func LastChar(str string) (string) {
	characters := strings.Split(str, "")
	return characters[len(characters) - 1]
}

func TerminateLineWithDotSpace(str string) (string) {
	if LastChar(str) == "." {
		str = str + " "
	}

	return str
}

func (d *drawer) Snippets(text string) ([][]string) {
	codeSnippetStart := "{{{{{{"
	codeSnippetEnd := "}}}}}}"
	textSnippetStart := "[[[[[["
	textSnippetEnd := "]]]]]]"

	snippets := make([][]string, 0)

	lines := Lines(text)

	accumulateCodeSnippet := false
	accumulateTextSnippet := false
	codeSnippet := make([]string, 0)
	textSnippet := make([]string, 0)

	for _, line := range lines {
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

		//This should be after the `codeSnippetEnd` check!
		if accumulateCodeSnippet {
			codeSnippet = append(codeSnippet, line)
			continue
		}

		if line == textSnippetStart {
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

		line = TerminateLineWithDotSpace(line)

		sentences := Sentences(line)
		for _, sentence := range sentences {
			// sentence = sentence + "."

			// Should we be splitting this sentence up?
			if (len(WordsLongerThan2LettersIn(sentence)) > 10) {
				fmt.Printf("SPLITTING sentence <<%s>> as it is larger than 10 words.\n", sentence)
				// Split this sentence further. Try splitting by commas. [TODO - semi-colons]
				phrases := Phrases(sentence)
				for index, phrase := range phrases {
					if index != len(phrases) - 1 || LastChar(sentence) == "," {
						phrase = phrase + ","
					}

					// The first phrase of a new sentence, even if it is small, should not be considered for clubbing with the previous sentence.
					isFirstPhraseOfThisSentence := index == 0

					// Should we be splitting this phrase up?
					if (len(WordsLongerThan2LettersIn(phrase)) > 10) {
						fmt.Printf("SPLITTING phrase <<%s>> as it is larger than 10 words.\n", phrase)
						phraseParts := minimumPhrasePartsForWordsPerPhrasePartLessThan(phrase, 10)
						wordsInPhrase := Words(phrase)
						wordsPerPhrasePart := len(wordsInPhrase) / phraseParts

						// The first phrase part of a phrase, even if it is small, should not be considered for clubbing with the previous sentence.
						isFirstPhrasePartOfThisPhrase := true

						for len(wordsInPhrase) > 0 {
							upperBoundIndex := wordsPerPhrasePart
							if wordsPerPhrasePart > len(wordsInPhrase) {
								upperBoundIndex = len(wordsInPhrase)
							}
							phrasePart := strings.Join(wordsInPhrase[0 : upperBoundIndex], " ")

							isFirstPhrasePartOfThisSentence := isFirstPhraseOfThisSentence && isFirstPhrasePartOfThisPhrase
							if !isFirstPhrasePartOfThisSentence && ShouldClubWithPreviousText(textSnippet, phrasePart) {
								fmt.Printf("CLUBBING phrase part <<%s>> with previous text.\n", phrasePart)
								ClubWithPreviousText(textSnippet, phrasePart, " ")
							} else {
								fmt.Printf("DID NOT CLUB phrase part <<%s>> with previous text.\n", phrasePart)
								textSnippet = append(textSnippet, phrasePart)
							}

							wordsInPhrase = wordsInPhrase[upperBoundIndex:]
							isFirstPhrasePartOfThisPhrase = false
						}
					} else {
						if !isFirstPhraseOfThisSentence && ShouldClubWithPreviousText(textSnippet, phrase) {
							fmt.Printf("CLUBBING phrase <<%s>> with previous text.\n", phrase)
							ClubWithPreviousText(textSnippet, phrase, "")
						} else {
							fmt.Printf("DID NOT CLUB phrase <<%s>> with previous text.\n", phrase)
							textSnippet = append(textSnippet, phrase)
						}
					}
				}
			} else {
				// Skip the "Club with previous sentence" optimization for sentences. Do it only for phrases, as above!
				textSnippet = append(textSnippet, sentence)
				// if len(textSnippet) > 0 && (len(WordsLongerThan2Letters(textSnippet[len(textSnippet) - 1])) + len(WordsLongerThan2Letters(sentence))) <= 13 {
				// if ShouldClubWithPreviousText(textSnippet, sentence) {
				// 	ClubWithPreviousText(textSnippet, sentence, " ")
				// } else {
				// 	textSnippet = append(textSnippet, sentence.Trim(" \t\n\r"))
				// }
			}
		}

		// If we are in the context of processing "Single Line Text".
		// Processing of "Text Snippet Accumulation" is being handled in `textSnippetEnd` check above.
		if accumulateTextSnippet == false {
			snippets = append(snippets, textSnippet)
			textSnippet = make([]string, 0)
		}
	}

	PrintSnippets(snippets)
	return snippets
}

func ClubWithPreviousText(snippet []string, text string, separator string) {
	// text = strings.Trim(text, " \t\n\r")
	snippet[len(snippet) - 1] = snippet[len(snippet) - 1] + separator + text
}

func ShouldClubWithPreviousText(snippet []string, text string) (bool) {
	if len(snippet) == 0 {
		return false
	}

	previousText := snippet[len(snippet) - 1]

	return len(WordsLongerThan2LettersIn(previousText)) + len(WordsLongerThan2LettersIn(text)) < 8
}

func minimumPhrasePartsForWordsPerPhrasePartLessThan(phrase string, maximumWordsPerPhrasePart int) (int) {
	wordsInPhrase := Words(phrase)
	minimumPhraseParts := 2;
	for len(wordsInPhrase) / minimumPhraseParts > maximumWordsPerPhrasePart {
		minimumPhraseParts = minimumPhraseParts + 1
	}

	return minimumPhraseParts
}

func PrintSnippets(snippets [][]string) {
	for _, snippet := range snippets {
		fmt.Println("Snippet")
		for _, line := range snippet {
			fmt.Printf("Line: %s\n", line)
		}
		fmt.Println("")
	}
}

func IsPlaceHolderImageCommand(snippet []string) (bool) {
	return len(snippet) > 0 && strings.HasPrefix(snippet[0], "PLACEHOLDER_IMAGE ")
}

func LeftPad2Len(s string, padStr string, overallLen int) string {
	var padCount = overallLen - len(s)
	if padCount > 0 {
		return strings.Repeat(padStr, padCount) + s
	} else {
		return s
	}
}

// Draw returns the image of a text
func (d *drawer) Draw(text string) {
	snippets := d.Snippets(text)

	overallLenForPadding := len(strconv.Itoa(len(snippets) - 1))

	for index, snippet := range snippets {
		if len(snippet) == 0 {
			continue
		}

		var bgColor, textColor color.RGBA

		//set unique random color every time
		d.SetColors(bgColor, textColor)
		//let it use auto font size
		d.SetFontSize(0)

		fileName := LeftPad2Len(strconv.Itoa(index), "0", overallLenForPadding) + ".jpg"
		if IsPlaceHolderImageCommand(snippet) {
			d.bringInPlaceholderImageToItsRightPlace(snippet[0], fileName)
		} else {
			d.drawSnippet(snippet, filepath.Join(d.OutputFolder, fileName))
		}
	}
}

func (d *drawer) drawBackgroundImage() (*image.RGBA) {
	var img *image.RGBA

	if d.BackgroundImage != nil {
		imgRect := image.Rectangle{image.Pt(0, 0), d.BackgroundImage.Bounds().Size()}
		img = image.NewRGBA(imgRect)
		draw.Draw(img, img.Bounds(), d.BackgroundImage, image.ZP, draw.Src)
	} else {
		img = image.NewRGBA(image.Rect(0, 0, d.Width, d.Height))
		draw.Draw(img, img.Bounds(), d.BackgroundColor, image.ZP, draw.Src)
	}

	return img
}

func setContextProperties(c *freetype.Context, d *drawer, img *image.RGBA) {
	c.SetDPI(72)
	c.SetFont(d.Font)
	c.SetFontSize(d.FontSize)
	c.SetClip(img.Bounds())
	c.SetDst(img)
	c.SetSrc(d.TextColor)
	c.SetHinting(font.HintingNone)
}

func (d *drawer) drawSnippet(lines []string, output string) {
	var err error

	//Calculate the minimum font fize considering all the text lines in the snippet
	if d.autoFontSize {
		d.FontSize = d.calcFontSizeForMultipleLines(lines)
	}

	var img *image.RGBA = d.drawBackgroundImage()
	
	if d.Font != nil {
		c := freetype.NewContext()
		setContextProperties(c, d, img)

		gapFromLastLine := 0
		textHeight := int(c.PointToFixed(d.FontSize) >> 6)
		startingHeightPoint := (d.Height - len(lines) * textHeight - (len(lines) - 1) * 40) / 2

		for _, line := range lines {
			line = strings.Trim(line, " \t\n\r")
			if line == "" {
				continue
			}
			// line = line + "."
			// pt := freetype.Pt((d.Width-textWidth)/2+d.TextPosHorizontal, (d.Height+textHeight)/2+d.TextPosVertical + gapFromLastLine)

			// Use the below one for center alignment of text
			textWidth := d.calcTextWidth(d.FontSize, line)
			pt := freetype.Pt((d.Width-textWidth)/2+d.TextPosHorizontal, startingHeightPoint + gapFromLastLine)

			// Use the below one for left alignment of text
			// pt := freetype.Pt(d.TextPosHorizontal, startingHeightPoint + gapFromLastLine)

			gapFromLastLine += textHeight + 40
			_, err = c.DrawString(line, pt)
		}
	}

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

func (d *drawer) bringInPlaceholderImageToItsRightPlace(snippetLine string, fileName string) {
	placeholderFilename := strings.Replace(snippetLine, "PLACEHOLDER_IMAGE ", "", -1)
	fmt.Printf("PLACEHOLDER FILE NAME = %s\n", placeholderFilename)
	if strings.HasSuffix(placeholderFilename, ".png") {
		fileName = strings.Replace(fileName, ".jpg", ".png", -1)
	}
	os.Rename(filepath.Join(d.OutputFolder, placeholderFilename), filepath.Join(d.OutputFolder, fileName))
}

// SetFontPos sets the fontPos
func (d *drawer) SetTextPos(textPosVertical, textPosHorizontal int) {
	d.TextPosVertical = textPosVertical
	d.TextPosHorizontal = textPosHorizontal
	if d.TextPosHorizontal == 0 {
		d.TextPosHorizontal = 10
	}
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

func (d *drawer) calcFontSizeForSingleLine(text string) (fontSize float64) {
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

func (d *drawer) calcFontSizeForMultipleLines(lines []string) (float64) {
	var minFontSize float64 = 10000
	for _, line := range lines {
		minFontSize = math.Min(minFontSize, d.calcFontSizeForSingleLine(line))
	}
	return minFontSize
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
