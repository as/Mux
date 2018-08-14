package main

import (
	"bytes"
	"fmt"
	"image"
	"image/draw"
	"io"
	"log"
	"os"
	"os/exec"

	"github.com/as/font"
	"github.com/as/frame"
	"github.com/as/shiny/event/lifecycle"
	"github.com/as/shiny/event/paint"
	"github.com/as/ui"
	"github.com/as/ui/tag"
)

var winSize = image.Pt(800, 1440)

type Con struct {
	fr  *frame.Frame
	kbd chan []byte
	sp  image.Point
}

func ck(err error) {
	if err != nil {
		log.Fatalln(err)
	}
}

type InsertEvent struct {
	f      *tag.Tag
	q0, q1 int64
	p      []byte
}
type StdinEvent struct {
	in io.WriteCloser
	p  []byte
}

var repaint func()

func main() {
	dev, err := ui.Init(&ui.Config{Width: winSize.X, Height: winSize.Y, Title: "mux"})
	if err != nil {
		log.Fatalln(err)
	}

	D := dev.Window().Device()
	repaint = func() {
		select {
		case D.Paint <- paint.Event{}:
		default:
		}
	}
	ctl := make(chan interface{})
	wind := dev.Window()
	t := tag.New(dev, &tag.Config{
		Facer:      font.NewGoMono,
		FaceHeight: 11,
		Ctl:        ctl,
	})

	t.Resize(winSize)
	t.Refresh()
	repaint()

	active := t.Window
	var qcmd int64
	selectwin := func(mouse image.Point) {
		active = t.Window
		if mouse.In(t.Label.Bounds()) {
			active = t.Label
		}
	}

	if len(os.Args) <= 1 {
		panic("less than 1 arg")
	}
	os.Args = os.Args[1:]
	cmd := exec.Command(os.Args[0], os.Args[1:]...)
	go func() {
		if err := cmd.Start(); err != nil {
			panic(err)
		}
		println("program dies")
	}()
	qcmd = int64(len(t.Bytes()))

	fmt.Fprintln(t, "hello")
	go func() {
		for {
			select {
			case e := <-D.Key:
				if e.Direction == 2 {
					continue
				}
				if e.Rune == '\r' {
					e.Rune = '\n'
				}
				l0 := int64(len(active.Bytes()))
				//			t.Handle(t.Body, e)
				_, q1 := active.Dot()
				t.Insert([]byte(string(e.Rune)), q1)
				repaint()
				if q1 > qcmd-1 {
					if e.Rune == '\n' {
						p := append([]byte{}, t.Bytes()[qcmd:q1]...)
						qcmd = q1
						ctl <- StdinEvent{t, p}
					}
				} else if l1 := int64(len(active.Bytes())); l0 > l1 {
					qcmd -= (l0 - l1)
				} else {
					qcmd++
				}
			}
		}
	}()

	for {
		select {
		case e := <-ctl:
			switch e := e.(type) {
			case StdinEvent:
				io.Copy(e.in, bytes.NewReader(e.p))
			case InsertEvent:
				e.f.Insert(e.p, qcmd)
				qcmd += int64(len(e.p))
			}
			repaint()
		case e := <-D.Mouse:
			selectwin(image.Pt(int(e.X), int(e.Y)))
			e.X -= float32(t.Bounds().Min.X)
			e.Y -= float32(t.Bounds().Min.Y)
			//			t.Handle(active, e)
		case e := <-D.Size:
			winSize.X = e.WidthPx
			winSize.Y = e.HeightPx
			t.Resize(winSize)
			repaint()
		case <-D.Paint:
			t.Upload()
			wind.Publish()
		case e := <-D.Lifecycle:
			if e.To == lifecycle.StageDead {
				return
			}
		}
	}
}

var paintcnt = 0

func drawBorder(dst draw.Image, r image.Rectangle, src image.Image, sp image.Point, thick int) {
	draw.Draw(dst, image.Rect(r.Min.X, r.Min.Y, r.Max.X, r.Min.Y+thick), src, sp, draw.Src)
	draw.Draw(dst, image.Rect(r.Min.X, r.Max.Y-thick, r.Max.X, r.Max.Y), src, sp, draw.Src)
	draw.Draw(dst, image.Rect(r.Min.X, r.Min.Y, r.Min.X+thick, r.Max.Y), src, sp, draw.Src)
	draw.Draw(dst, image.Rect(r.Max.X-thick, r.Min.Y, r.Max.X, r.Max.Y), src, sp, draw.Src)
}
