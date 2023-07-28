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

func drawText(screen tcell.Screen, text string, posX int, posY int) {
	for _, r := range text {
		screen.SetContent(posX, posY, r, nil, tcell.StyleDefault)
		posX++
	}
}

func (ap *audioPanel) render(screen tcell.Screen) {
	drawText(screen, "ciaoo", 10, 10)
}

func (ap *audioPanel) handleEvent(event tcell.Event) (bool, bool) {
	switch event := event.(type) {
	case *tcell.EventKey:
		if event.Key() == tcell.KeyESC {
			return false, true
		}

		if event.Key() != tcell.KeyRune {

			return false, false
		}

		if event.Key() == tcell.KeyUp {
			speaker.Lock()
			ap.volume.Volume += 0.2
			speaker.Unlock()
			return true, false
		}

		if event.Key() == tcell.KeyDown {
			speaker.Lock()
			ap.volume.Volume -= 0.2
			speaker.Unlock()
			return true, false
		}

		switch unicode.ToLower(event.Rune()) {
		case 'q':
			return false, true
		case 'a':
			speaker.Lock()
			if ap.volume.Volume < 2.5 {
				ap.volume.Volume += 0.1
			}
			speaker.Unlock()
			return true, false
		case 'd':
			speaker.Lock()
			if ap.volume.Volume > -16 {
				ap.volume.Volume -= 0.1
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
				newPos += ap.sampleRate.N(time.Second)
			}

			if event.Rune() == 'b' {
				newPos -= ap.sampleRate.N(time.Second)
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
	defer file.Close()

	fileStream, format, err := mp3.Decode(file)
	if err != nil {
		fmt.Println(fmt.Sprintf("Error while decoding mp3 file: %s\n", err))
		os.Exit(1)
	}
	defer fileStream.Close()

	fmt.Println("Channels -> ", format.NumChannels)
	fmt.Println("Sample rate -> ", format.SampleRate)

	err = speaker.Init(format.SampleRate, format.SampleRate.N(time.Second/30))
	if err != nil {
		fmt.Println(fmt.Sprintf("Error while initing the speaker: %s \n", err))
		os.Exit(1)
	}
	defer speaker.Close()

	screen, error := tcell.NewScreen()
	if error != nil {
		fmt.Println(fmt.Sprintf("Error opening screen %s\n", error))
		panic(1)
	}

	error = screen.Init()
	if error != nil {
		fmt.Println(fmt.Sprintf("Error initing screen: %s\n", error))
		panic(1)
	}
	defer screen.Fini()

	ap := newAudioPanel(format.SampleRate, fileStream)

	screen.Clear()
	screen.Show()
	ap.play()

	events := make(chan tcell.Event)
	seconds := time.Tick(time.Second)

	go func() {
		for {
			events <- screen.PollEvent()
		}
	}()

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
