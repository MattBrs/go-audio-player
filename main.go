package main

import (
	"fmt"
	"os"
	"time"
	"unicode"

	"github.com/faiface/beep"
	"github.com/faiface/beep/effects"
	"github.com/faiface/beep/mp3"
	"github.com/faiface/beep/speaker"
	"github.com/gdamore/tcell"
)

const MAXVOLUME = 2.0
const MINVOLUME = -10.0

type audioPanel struct {
	sampleRate beep.SampleRate
	fileStream beep.StreamSeeker
	volume     *effects.Volume
	ctrl       *beep.Ctrl
	resampler  *beep.Resampler
}

func (ap *audioPanel) play() {
	speaker.Play(ap.volume)
}

func drawText(screen tcell.Screen, text string, posX int, posY int, style tcell.Style) {
	for _, r := range text {
		screen.SetContent(posX, posY, r, nil, style)
		posX++
	}
}

func calcVolumePercentage(volume float64) string {
	normalizedVolume := int32(((volume - MINVOLUME) / (MAXVOLUME - MINVOLUME)) * 100)
	return fmt.Sprintf("%d", normalizedVolume)
}

func calcComplPercentage(position float64, length float64) float64 {
	if position == 0 {
		return 0
	}

	return (position * 100) / length
}

func drawPercentageBar(screen tcell.Screen, posX int, posY int, percentage int64, style tcell.Style) {
	drawText(screen, "[", posX, posY, style)

	var mutPercentage int = int(percentage) / 2

	for i := 0; i < mutPercentage; i++ {
		drawText(screen, "-", posX+i+1, int(posY), style)
	}

	drawText(screen, "]", posX+50, posY, style)
}

func (ap *audioPanel) render(screen tcell.Screen) {
	screen.Clear()
	mainStyle := tcell.StyleDefault.
		Background(tcell.NewRGBColor(71, 52, 55)).
		Foreground(tcell.NewRGBColor(215, 216, 162))
	statusStyle := mainStyle.
		Foreground(tcell.NewRGBColor(221, 216, 16)).
		Bold(true)
	screen.Fill(' ', mainStyle)

	speaker.Lock()
	currentVolume := ap.volume.Volume
	isPaused := fmt.Sprintf("%t", ap.ctrl.Paused)
	position := ap.sampleRate.D(ap.fileStream.Position())
	length := ap.sampleRate.D(ap.fileStream.Len())
	speaker.Unlock()

	elapsed := fmt.Sprintf("%v", position.Round(time.Second))
	trackLength := fmt.Sprintf("%v", length.Round(time.Second))
	drawText(screen, "current volume", 0, 0, mainStyle)
	drawText(screen, calcVolumePercentage(currentVolume)+"%", 0, 1, statusStyle)

	drawText(screen, "paused", 20, 0, mainStyle)
	drawText(screen, isPaused, 20, 1, statusStyle)

	drawText(screen, "["+elapsed, 12, 9, mainStyle)
	drawText(screen, "/", 21, 9, mainStyle)
	drawText(screen, trackLength+"]", 26, 9, mainStyle)

	drawText(screen, "percentage completed", 80, 0, mainStyle)

	complPerc := calcComplPercentage(position.Seconds(), length.Seconds())

	drawText(
		screen,
		fmt.Sprintf("%d", int64(complPerc)),
		80,
		1,
		statusStyle)

	drawPercentageBar(screen, 0, 10, int64(complPerc), statusStyle)

	screen.Show()
}

func (ap *audioPanel) handleEvent(event tcell.Event) (bool, bool) {
	switch event := event.(type) {
	case *tcell.EventKey:
		if event.Key() == tcell.KeyESC {
			return false, true
		}

		// check if the pressed key is a unicode key
		if event.Key() != tcell.KeyRune {
			return false, false
		}

		switch unicode.ToLower(event.Rune()) {
		case 'q':
			return false, true
		case 'a':
			speaker.Lock()

			ap.volume.Volume += 0.4
			if ap.volume.Volume >= MAXVOLUME {
				ap.volume.Volume = MAXVOLUME
			}
			speaker.Unlock()
			return true, false
		case 'd':
			speaker.Lock()

			ap.volume.Volume -= 0.6
			if ap.volume.Volume <= MINVOLUME {
				ap.volume.Volume = MINVOLUME
			}

			speaker.Unlock()
			return true, false
		case 'p':
			speaker.Lock()
			ap.ctrl.Paused = !ap.ctrl.Paused
			speaker.Unlock()
			return true, false
		case 'n', 'b':
			speaker.Lock()

			newPos := ap.fileStream.Position()
			if event.Rune() == 'n' {
				newPos += 5 * ap.sampleRate.N(time.Second)
			}

			if event.Rune() == 'b' {
				newPos -= 5 * ap.sampleRate.N(time.Second)
			}

			if newPos < 0 {
				newPos = 0
			}

			if newPos >= ap.fileStream.Len() {
				newPos = ap.fileStream.Len() - 1
			}

			error := ap.fileStream.Seek(newPos)
			if error != nil {
				fmt.Println(fmt.Sprintf("Error while changing song position: %s", error))
			}
			speaker.Unlock()
		}

	}

	return false, false
}

func newAudioPanel(sampleRate beep.SampleRate, stream beep.StreamSeeker) *audioPanel {
	ctrl := &beep.Ctrl{Streamer: beep.Loop(1, stream)}
	resampler := beep.ResampleRatio(4, 1, ctrl)
	volume := &effects.Volume{Streamer: resampler, Base: 2}
	return &audioPanel{sampleRate: sampleRate, resampler: resampler, volume: volume, ctrl: ctrl, fileStream: stream}
}

func run(screen tcell.Screen, ap *audioPanel) {
	events := make(chan tcell.Event)  // create channel to handle user input events
	seconds := time.Tick(time.Second) // create channel that gets updated every second

	// gorouting to grap all user input
	go func() {
		for {
			events <- screen.PollEvent()
		}
	}()

	// when an event happens, execute the relative procedure
	for true {
		select {
		case event := <-events:
			changed, quit := ap.handleEvent(event)
			if quit {
				return
			}

			if changed {
				ap.render(screen)
			}

		case <-seconds:
			ap.render(screen)
		}
	}
}

func initScreen() tcell.Screen {
	screen, error := tcell.NewScreen()
	if error != nil {
		fmt.Println(fmt.Sprintf("Error opening screen %s\n", error))
		os.Exit(1)
	}

	error = screen.Init()
	if error != nil {
		fmt.Println(fmt.Sprintf("Error initing screen: %s\n", error))
		os.Exit(1)
	}

	return screen
}

func decodeFile(file *os.File) (beep.StreamSeekCloser, beep.Format) {
	fileStream, format, err := mp3.Decode(file)
	if err != nil {
		fmt.Println(fmt.Sprintf("Error while decoding mp3 file: %s\n", err))
		os.Exit(1)
	}

	return fileStream, format
}

func initSpeaker(format beep.Format) {
	error := speaker.Init(format.SampleRate, format.SampleRate.N(time.Second/30))
	if error != nil {
		fmt.Println(fmt.Sprintf("Error while initing the speaker: %s \n", error))
		os.Exit(1)
	}
}

func main() {
	if len(os.Args) != 2 {
		fmt.Println("Wrong number of parameters! Specify only the filename.")
		os.Exit(1)
	}

	file, error := os.Open(os.Args[1])
	if error != nil {
		fmt.Println(fmt.Sprintf("Error while opening file: %\n", error))
		os.Exit(1)
	}

	fileStream, format := decodeFile(file)

	initSpeaker(format)
	screen := initScreen()
	ap := newAudioPanel(format.SampleRate, fileStream)

	ap.render(screen)
	ap.play()

	run(screen, ap)

	defer fileStream.Close()
	defer file.Close()
	defer speaker.Close()
	defer screen.Fini()
}
